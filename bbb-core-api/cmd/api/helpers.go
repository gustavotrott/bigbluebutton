package main

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"hash"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	bbbcore "github.com/bigbluebutton/bigbluebutton/bbb-core-api/gen/bbb-core"
	"github.com/bigbluebutton/bigbluebutton/bbb-core-api/gen/common"
	"github.com/bigbluebutton/bigbluebutton/bbb-core-api/internal/model"
	"github.com/bigbluebutton/bigbluebutton/bbb-core-api/internal/util"
	"github.com/bigbluebutton/bigbluebutton/bbb-core-api/internal/validation"
	"github.com/thanhpk/randstr"
)

const (
	breakoutRoomsCaptureSlides         = false
	breakoutRoomsCaptureNotes          = false
	breakoutRoomsCaptureSlidesFileName = "%%CONFNAME%%"
	breakoutRoomsCaptureNotesFileName  = "%%CONFNAME%%"
	dialNum                            = "%%DIALNUM%%"
	confNum                            = "%%CONFNUM%%"
	confName                           = "%%CONFNAME%%"
	serverUrl                          = "%%SERVERURL%%"
)

func (app *Config) writeXML(w http.ResponseWriter, status int, data any, headers ...http.Header) error {
	xml, err := xml.Marshal(data)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(status)

	_, err = w.Write(xml)
	if err != nil {
		return err
	}

	return nil
}

func (app *Config) isChecksumValid(r *http.Request, apiCall string) (bool, string, string) {
	params := r.URL.Query()

	if app.Security.Salt == "" {
		log.Println("Security is disabled in this service. Make sure this is intentional.")
		return true, "", ""
	}

	checksum := params.Get("checksum")
	if checksum == "" {
		return false, model.ChecksumErrorKey, model.ChecksumErrorMsg
	}

	queryString := r.URL.RawQuery
	queryWithoutChecksum := app.removeQueryParam(queryString, "checksum")
	log.Printf("Query string after checksum removed [%s]\n", queryWithoutChecksum)

	data := apiCall + queryWithoutChecksum + app.Security.Salt
	var createdChecksum string

	switch checksumLength := len(checksum); checksumLength {
	case 40:
		_, ok := app.ChecksumAlgorithms["sha-1"]
		if ok {
			createdChecksum = app.generateHashString(data, sha1.New())
			log.Println("SHA-1", createdChecksum)
		}
	case 64:
		_, ok := app.ChecksumAlgorithms["sha-256"]
		if ok {
			createdChecksum = app.generateHashString(data, sha256.New())
			log.Println("SHA-256", createdChecksum)
		}
	case 96:
		_, ok := app.ChecksumAlgorithms["sha-384"]
		if ok {
			createdChecksum = app.generateHashString(data, sha512.New384())
			log.Println("SHA-384", createdChecksum)
		}
	case 128:
		_, ok := app.ChecksumAlgorithms["sha-512"]
		if ok {
			createdChecksum = app.generateHashString(data, sha512.New())
			log.Println("SHA-512", createdChecksum)
		}
	default:
		log.Println("No algorithm could be found that matches the provided checksum length")
	}

	if createdChecksum == "" || createdChecksum != checksum {
		log.Printf("checksumError: Query string checksum failed. Our: [%s], Client: [%s]", createdChecksum, checksum)
		return false, model.ChecksumErrorKey, model.ChecksumErrorMsg
	}

	return true, "", ""
}

func (app *Config) generateHashString(data string, h hash.Hash) string {
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func (app *Config) removeQueryParam(queryString, param string) string {
	entries := strings.Split(queryString, "&")
	var newEntries []string

	for _, entry := range entries {
		if kv := strings.SplitN(entry, "=", 2); kv[0] != param {
			newEntries = append(newEntries, entry)
		}
	}

	return strings.Join(newEntries, "&")
}

func (app *Config) processCreateQueryParams(params *url.Values) (*common.MeetingSettings, error) {
	var settings common.MeetingSettings

	createTime := time.Now().UnixMilli()

	isBreakout := util.GetBoolOrDefaultValue(params.Get("isBreakout"), false)
	var parentMeetingInfo *common.MeetingInfo
	if isBreakout {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		res, err := app.BbbCore.GetMeetingInfo(ctx, &bbbcore.MeetingInfoRequest{
			MeetingId: util.GetStringOrDefaultValue(params.Get("parentMeetingID"), ""),
		})
		if err != nil {
			log.Println(err)
			return nil, err
		}
		parentMeetingInfo = res.MeetingInfo
		settings.BreakoutProps = app.processBreakoutProps(params, parentMeetingInfo)
	}

	learningDashboardEnabled := false
	settings.MeetingProps, learningDashboardEnabled = app.processMeetingProps(params, createTime, isBreakout, parentMeetingInfo)
	settings.DurationProps = app.processDurationProps(params, createTime)
	settings.PasswordProps = app.processPasswordProps(params, learningDashboardEnabled)
	settings.RecordProps = app.processRecordProps(params)

	voiceProps, err := app.processVoiceProps(params)
	if err != nil {
		return nil, err
	}

	settings.VoiceProps = voiceProps
	settings.WelcomeProps = app.processWelcomeProps(params, isBreakout, settings.VoiceProps.DialNumber, settings.VoiceProps.VoiceBridge, settings.MeetingProps.Name)
	settings.UsersProps = app.processUsersProps(params)
	settings.MetadataProps = app.processMetadataProps(params)
	settings.LockSettingsProps = app.processLockSettingsProps(params)
	settings.SystemProps = app.processSystemProps(params)
	settings.GroupProps = app.processGroupProps(params)

	/* TODO: Implement client settings override */

	return &settings, nil
}

func (app *Config) processMeetingProps(params *url.Values, createTime int64, isBreakout bool, parentMeetingInfo *common.MeetingInfo) (*common.MeetingProps, bool) {
	var meetingIntId string
	var meetingExtId string

	if isBreakout {
		meetingIntId = validation.StripCtrlChars(params.Get("meetingID"))
		parentMeetingId := parentMeetingInfo.MeetingIntId
		parentCreateTime := parentMeetingInfo.DurationInfo.CreateTime
		data := parentMeetingId + "-" + strconv.Itoa(int(parentCreateTime)) + "-" + validation.StripCtrlChars(params.Get("sequence"))
		meetingExtId = app.generateHashString(data, sha1.New()) + strconv.Itoa(int(createTime))
	} else {
		meetingExtId = validation.StripCtrlChars(params.Get("meetingID"))
		meetingIntId = app.generateHashString(meetingExtId, sha1.New()) + strconv.Itoa(int(createTime))
	}

	disabledFeaturesMap := make(map[string]struct{})
	for k, v := range app.DisabledFeatures {
		disabledFeaturesMap[k] = v
	}
	if featuresParam := params.Get("disabledFeatures"); featuresParam != "" {
		features := strings.Split(featuresParam, ",")
		for _, feature := range features {
			disabledFeaturesMap[feature] = struct{}{}
		}
	}
	disabledFeatures := make([]string, len(disabledFeaturesMap))
	i := 0
	for feature := range disabledFeaturesMap {
		disabledFeatures[i] = feature
		i++
	}

	learningDashboardEnabled := false
	_, ok := disabledFeaturesMap["learningDashboard"]
	if !ok {
		learningDashboardEnabled = true
	}

	return &common.MeetingProps{
		Name:                validation.StripCtrlChars(params.Get("name")),
		MeetingExtId:        meetingExtId,
		MeetingIntId:        meetingIntId,
		MeetingCameraCap:    util.GetInt32OrDefaultValue(params.Get("meetingCameraCap"), app.Meeting.Cameras.Cap),
		MaxPinnedCameras:    util.GetInt32OrDefaultValue(params.Get("maxPinnedCamera"), app.Meeting.Cameras.MaxPinned),
		IsBreakout:          isBreakout,
		DisabledFeatures:    disabledFeatures,
		NotifyRecordingIsOn: util.GetBoolOrDefaultValue(params.Get("notfiyRecordingIsOn"), app.Recording.NotifyRecordingIsOn),
		PresUploadExtDesc:   util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("presentationUploadExternalDescription")), app.Presentation.Upload.External.Description),
		PresUploadExtUrl:    util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("presentationUploadExternalUrl")), app.Presentation.Upload.External.Url),
	}, learningDashboardEnabled
}

func (app *Config) processBreakoutProps(params *url.Values, parentMeetingInfo *common.MeetingInfo) *common.BreakoutProps {
	return &common.BreakoutProps{
		ParentMeetingId:       parentMeetingInfo.MeetingIntId,
		Sequence:              util.GetInt32OrDefaultValue(params.Get("sequence"), 0),
		FreeJoin:              util.GetBoolOrDefaultValue(params.Get("freeJoin"), false),
		BreakoutRooms:         make([]string, 0),
		Record:                util.GetBoolOrDefaultValue(params.Get("breakoutRoomsRecord"), app.BreakoutRooms.Record),
		PrivateChatEnabled:    util.GetBoolOrDefaultValue(params.Get("breakoutRoomsPrivateChatEnabled"), app.BreakoutRooms.PrivateChatEnabled),
		CaptureNotes:          util.GetBoolOrDefaultValue(params.Get("breakoutRoomsCaptureNotes"), breakoutRoomsCaptureNotes),
		CaptureSlides:         util.GetBoolOrDefaultValue(params.Get("breakoutRoomsCaptureSlides"), breakoutRoomsCaptureSlides),
		CaptureNotesFileName:  util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("breakoutRoomsCaptureNotesFileName")), breakoutRoomsCaptureNotesFileName),
		CaptureSlidesFileName: util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("breakoutRoomsCaptureSlidesFileName")), breakoutRoomsCaptureSlidesFileName),
	}
}

func (app *Config) processDurationProps(params *url.Values, createTime int64) *common.DurationProps {
	return &common.DurationProps{
		Duration:                           int32(util.GetInt32OrDefaultValue(params.Get("duration"), app.Meeting.Duration)),
		CreateTime:                         createTime,
		CreateDate:                         time.UnixMilli(createTime).String(),
		MeetingExpNoUserJoinedInMin:        util.GetInt32OrDefaultValue(params.Get("meetingExpireIfNoUserJoinedInMinutes"), app.Meeting.Expiry.NoUserJoined),
		MeetingExpLastUserLeftInMin:        util.GetInt32OrDefaultValue(params.Get("meetingExpireWhenLastUserLeftInMinutes"), app.Meeting.Expiry.LastUserLeft),
		UserInactivityInspectTimeInMin:     app.User.Inactivity.InspectInterval,
		UserInactivityThresholdInMin:       app.User.Inactivity.Threshold,
		UserActivitySignResponseDelayInMin: app.User.Inactivity.ResponseDelay,
		EndWhenNoMod:                       util.GetBoolOrDefaultValue(params.Get("endWhenNoModerator"), app.Meeting.Expiry.EndWhenNoModerator),
		EndWhenNoModDelayInMin:             util.GetInt32OrDefaultValue(params.Get("endWhenNoModeratorDelayInMinutes"), app.Meeting.Expiry.EndWhenNoModeratorDelay),
		LearningDashboardCleanupDelayInMin: util.GetInt32OrDefaultValue(params.Get("learningDashboardCleanupDelayInMinutes"), app.LearningDashboard.CleanupDelay),
	}
}

func (app *Config) processPasswordProps(params *url.Values, learningDashboardEnabled bool) *common.PasswordProps {
	learningDashboardAccessToken := ""
	if learningDashboardEnabled {
		learningDashboardAccessToken = randstr.String(12)
	}

	return &common.PasswordProps{
		ModeratorPw:                  util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("moderatorPW")), ""),
		AttendeePw:                   util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("attendeePW")), ""),
		LearningDashboardAccessToken: learningDashboardAccessToken,
	}
}

func (app *Config) processRecordProps(params *url.Values) *common.RecordProps {
	record := false
	if !app.Recording.Disabled {
		record = util.GetBoolOrDefaultValue(params.Get("record"), false)
	}

	return &common.RecordProps{
		Record:                  record,
		AutoStartRecording:      util.GetBoolOrDefaultValue(params.Get("autoStartRecording"), app.Recording.AutoStart),
		AllowStartStopRecording: util.GetBoolOrDefaultValue(params.Get("allowStartStopRecording"), app.Recording.AllowStartStopRecording),
		RecordFullDurationMedia: util.GetBoolOrDefaultValue(params.Get("recordFullDurationMedia"), app.Recording.RecordFullDurationMedia),
		KeepEvents:              util.GetBoolOrDefaultValue(params.Get("meetingKeepEvents"), app.Recording.KeepEvents),
	}
}

func (app *Config) processVoiceProps(params *url.Values) (*common.VoiceProps, error) {
	voiceBridge := util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("voiceBridge")), "")
	if voiceBridge == "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		res, err := app.BbbCore.GenerateVoiceBridge(ctx, &bbbcore.GenerateVoiceBridgeRequest{Length: app.Meeting.Voice.VoiceBridgeLength})
		if err != nil {
			log.Println(err)
			return nil, err
		}
		voiceBridge = res.VoiceBridge
	}

	return &common.VoiceProps{
		VoiceBridge: voiceBridge,
		VoiceConf:   voiceBridge,
		DialNumber:  util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("dialNumber")), app.Meeting.Voice.DialAccessNumber),
		MuteOnStart: util.GetBoolOrDefaultValue(params.Get("muteOnStart"), app.Meeting.Voice.MuteOnStart),
	}, nil
}

func (app *Config) processWelcomeProps(params *url.Values, isBreakout bool, dialNumber string, voiceBridge string, meetingName string) *common.WelcomeProps {
	welcomeMessageTemplate := util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("welcome")), app.Meeting.Welcome.Message.Template)
	if app.Meeting.Welcome.Message.Footer != "" && isBreakout {
		welcomeMessageTemplate += "<br><br>" + app.Meeting.Welcome.Message.Footer
	}

	welcomeMessage := replaceKeywords(welcomeMessageTemplate, dialNumber, voiceBridge, meetingName, app.Server.BigBlueButton.Url)

	modOnlyMsg := util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("moderatorOnlyMessage")), "")
	if modOnlyMsg != "" {
		modOnlyMsg = replaceKeywords(modOnlyMsg, dialNumber, voiceBridge, meetingName, app.Server.BigBlueButton.Url)
	}

	return &common.WelcomeProps{
		WelcomeMsgTemplate: welcomeMessageTemplate,
		WelcomeMsg:         welcomeMessage,
		ModOnlyMsg:         modOnlyMsg,
	}
}

func (app *Config) processUsersProps(params *url.Values) *common.UsersProps {
	maxUserConcurentAccess := app.Meeting.Users.MaxConcurrentAccess
	if !app.Meeting.Users.AllowDuplicateExtUserId {
		maxUserConcurentAccess = 1
	}
	return &common.UsersProps{
		MaxUsers:                  util.GetInt32OrDefaultValue(params.Get("maxParticipants"), app.Meeting.Users.Max),
		MaxUserConcurrentAccesses: maxUserConcurentAccess,
		WebcamsOnlyForMod:         util.GetBoolOrDefaultValue(params.Get("webcamsOnlyForModerator"), app.Meeting.Cameras.ModOnly),
		UserCameraCap:             util.GetInt32OrDefaultValue(params.Get("userCameraCap"), app.User.Camera.Cap),
		GuestPolicy:               util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("guestPolicy")), app.Meeting.Users.GuestPolicy),
		MeetingLayout:             util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("meetingLayout")), app.Meeting.Layout),
		AllowModsUnmuteUsers:      util.GetBoolOrDefaultValue(params.Get("allowModsToUnmuteUsers"), app.Meeting.Users.AllowModsToUnmute),
		AllowModsEjectCameras:     util.GetBoolOrDefaultValue(params.Get("allowModsToEjectCameras"), app.Meeting.Cameras.AllowModsToEject),
		AuthenticatedGuest:        app.Meeting.Users.AuthenticatedGuest,
		AllowPromoteGuest:         util.GetBoolOrDefaultValue(params.Get("allowPromoteGuestToModerator"), app.Meeting.Users.AllowPromoteGuest),
	}
}

func (app *Config) processMetadataProps(params *url.Values) *common.MetadataProps {
	r, _ := regexp.Compile("meta_[a-zA-Z][a-zA-Z0-9-]*$")
	metadata := make(map[string]string)
	for k, v := range *params {
		if r.MatchString(k) {
			metadata[strings.ToLower(strings.TrimPrefix(k, "meta_"))] = v[0]
		}
	}
	return &common.MetadataProps{
		Metadata: metadata,
	}
}

func (app *Config) processLockSettingsProps(params *url.Values) *common.LockSettingsProps {
	disableNotes := util.GetBoolOrDefaultValue(params.Get("lockSettingsDisableNotes"), app.Meeting.Lock.Disable.Notes)
	disableNotes = util.GetBoolOrDefaultValue(params.Get("lockSettingsDisableNote"), disableNotes)

	return &common.LockSettingsProps{
		DisableCam:             util.GetBoolOrDefaultValue(params.Get("lockSettingsDisbaleCam"), app.Meeting.Lock.Disable.Cam),
		DisableMic:             util.GetBoolOrDefaultValue(params.Get("lockSettingsDisableMic"), app.Meeting.Lock.Disable.Mic),
		DisablePrivateChat:     util.GetBoolOrDefaultValue(params.Get("lockSettingsDisablePrivateChat"), app.Meeting.Lock.Disable.Chat.Private),
		DisablePublicChat:      util.GetBoolOrDefaultValue(params.Get("lockSettingsDisablePublicChat"), app.Meeting.Lock.Disable.Chat.Public),
		DisableNotes:           disableNotes,
		HideUserList:           util.GetBoolOrDefaultValue(params.Get("lockSettingsHideUserList"), app.Meeting.Lock.Hide.UserList),
		LockOnJoin:             util.GetBoolOrDefaultValue(params.Get("lockSettingsLockOnJoin"), app.Meeting.Lock.OnJoin),
		LockOnJoinConfigurable: util.GetBoolOrDefaultValue(params.Get("lockSettingsOnJoinConfigurable"), app.Meeting.Lock.OnJoinConfigurable),
		HideViewersCursor:      util.GetBoolOrDefaultValue(params.Get("lockSettingsHideViewersCursor"), app.Meeting.Lock.Hide.ViewersCursor),
		HideViewersAnnotation:  util.GetBoolOrDefaultValue(params.Get("lockSettingsHideViewersAnnotation"), app.Meeting.Lock.Hide.ViewersAnnotation),
	}
}

func (app *Config) processSystemProps(params *url.Values) *common.SystemProps {
	logoutUrl := util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("logoutURL")), "")
	defaultLogoutUrl := app.Server.BigBlueButton.LogoutUrl
	if logoutUrl == "" {
		if defaultLogoutUrl == "" || defaultLogoutUrl == "default" {
			logoutUrl = app.Server.BigBlueButton.Url
		} else {
			logoutUrl = defaultLogoutUrl
		}
	}

	customLogoUrl := util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("logo")), "")
	if customLogoUrl == "" && app.Server.BigBlueButton.Logo.UseDefaultLogo {
		customLogoUrl = app.Server.BigBlueButton.Logo.DefaultLogoUrl
	}

	return &common.SystemProps{
		LoginUrl:      util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("loginURL")), ""),
		LogoutUrl:     logoutUrl,
		CustomLogoUrl: customLogoUrl,
		BannerText:    util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("bannerText")), ""),
		BannerColour:  util.GetStringOrDefaultValue(validation.StripCtrlChars(params.Get("bannerColor")), ""),
	}
}

func (app *Config) processGroupProps(params *url.Values) []*common.GroupProps {
	type Group struct {
		Id     string   `json:"id"`
		Name   string   `json:"name"`
		Roster []string `json:"roster"`
	}

	var groups []Group
	if groupsParam := params.Get("groups"); groupsParam != "" {
		if err := json.Unmarshal([]byte(groupsParam), &groups); err != nil {
			log.Println("Error unmarshalling groups parameter", err)
			return nil
		}
	}

	groupProps := make([]*common.GroupProps, len(groups))
	for i, g := range groups {
		groupProps[i] = &common.GroupProps{
			GroupId:     g.Id,
			Name:        g.Name,
			UsersExtIds: g.Roster,
		}
	}
	return groupProps
}

func replaceKeywords(message string, dialNumber string, voiceBridge string, meetingName string, url string) string {
	keywords := []string{dialNum, confNum, confName, serverUrl}
	for _, v := range keywords {
		switch v {
		case dialNum:
			message = strings.ReplaceAll(message, dialNum, dialNumber)
		case confNum:
			message = strings.ReplaceAll(message, confNum, voiceBridge)
		case confName:
			message = strings.ReplaceAll(message, confName, meetingName)
		case serverUrl:
			message = strings.ReplaceAll(message, serverUrl, url)
		}
	}
	return message
}
