package streamingserver

import (
	"bytes"
	"encoding/json"
	"maps"
	"sync"
	"time"

	"bbb-graphql-middleware/internal/common"
)

func HandleUserMediaGroupStateEvtMsg(receivedMessage common.RedisMessage, browserConnectionsMutex *sync.RWMutex, browserConnections map[string]*common.BrowserConnection) {
	userId := receivedMessage.Core.Body["userId"].(string)
	groupId := receivedMessage.Core.Body["groupId"].(string)
	mediaType := receivedMessage.Core.Body["mediaType"].(string)
	sender := receivedMessage.Core.Body["sender"].(bool)
	receiver := receivedMessage.Core.Body["receiver"].(bool)
	active := receivedMessage.Core.Body["active"].(bool)
	removed := receivedMessage.Core.Body["removed"].(bool)

	now := time.Now().UTC()

	item := map[string]any{
		"userId":     userId,
		"groupId":    groupId,
		"mediaType":  mediaType,
		"sender":     sender,
		"receiver":   receiver,
		"active":     active,
		"removed":    removed,
		"updatedAt":  now.Format("2006-01-02T15:04:05.000Z"),
		"__typename": "user_mediaGroup_stream",
	}

	browserResponseData := map[string]any{
		"id":   QueryIdPlaceholder,
		"type": "next",
		"payload": map[string]any{
			"data": map[string]any{
				"user_mediaGroup_stream": []any{
					item,
				},
			},
		},
	}
	jsonDataNext, _ := json.Marshal(browserResponseData)

	browserConnectionsToSendData := make([]*common.BrowserConnection, 0)
	browserConnectionsMutex.RLock()
	for _, bc := range browserConnections {
		if bc.MeetingId == receivedMessage.Core.Header.MeetingId {
			browserConnectionsToSendData = append(browserConnectionsToSendData, bc)
		}
	}
	browserConnectionsMutex.RUnlock()

	for _, bc := range browserConnectionsToSendData {
		bc.ActiveStreamingsMutex.RLock()
		queryIds, existsUserMediaGroupStream := bc.ActiveStreamings["getUserMediaGroupStateStream"]
		bc.ActiveStreamingsMutex.RUnlock()
		if existsUserMediaGroupStream {
			for i := range queryIds {
				payload := bytes.Replace(jsonDataNext, QueryIdPlaceholderInBytes, []byte(queryIds[i]), 1)
				bc.FromHasuraToBrowserChannel.TrySend(payload)
			}
		}
	}

	if removed {
		RemoveUserMediaGroupStatesCache(
			receivedMessage.Core.Header.MeetingId,
			userId,
			groupId,
		)
	} else {
		StoreUserMediaGroupStatesCache(
			receivedMessage.Core.Header.MeetingId,
			userId,
			groupId,
			item,
		)
	}
}

func SendPreviousUserMediaGroupState(browserConnection *common.BrowserConnection, queryId string) {
	previousMessages, existsPreviousMessages := GetUserMediaGroupStatesCache(browserConnection.MeetingId)
	if existsPreviousMessages {
		items := make([]any, 0, len(previousMessages))
		for _, message := range previousMessages {
			items = append(items, message)
		}

		browserResponseData := map[string]any{
			"id":   queryId,
			"type": "next",
			"payload": map[string]any{
				"data": map[string]any{
					"user_mediaGroup_stream": items,
				},
			},
		}
		jsonDataNext, _ := json.Marshal(browserResponseData)
		browserConnection.FromHasuraToBrowserChannel.SendWait(browserConnection.Context, jsonDataNext)
	}
}

// Broadcasts a synthetic removal event for every cached entry of a given user
// in a meeting and clears them from the cache. Used when the user leaves the
// meeting and no explicit removal event has been emitted per (user, group).
func BroadcastRemoveAndClearUserMediaGroupCache(meetingId string, userId string, browserConnectionsMutex *sync.RWMutex, browserConnections map[string]*common.BrowserConnection) {
	cachedRowsForUser := PopUserMediaGroupStatesCacheForUser(meetingId, userId)
	if len(cachedRowsForUser) == 0 {
		return
	}

	now := time.Now().UTC()
	items := make([]any, 0, len(cachedRowsForUser))
	for _, row := range cachedRowsForUser {
		mediaType, _ := row["mediaType"].(string)
		groupId, _ := row["groupId"].(string)
		items = append(items, map[string]any{
			"userId":     userId,
			"groupId":    groupId,
			"mediaType":  mediaType,
			"sender":     false,
			"receiver":   false,
			"active":     false,
			"removed":    true,
			"updatedAt":  now.Format("2006-01-02T15:04:05.000Z"),
			"__typename": "user_mediaGroup_stream",
		})
	}

	browserResponseData := map[string]any{
		"id":   QueryIdPlaceholder,
		"type": "next",
		"payload": map[string]any{
			"data": map[string]any{
				"user_mediaGroup_stream": items,
			},
		},
	}
	jsonDataNext, _ := json.Marshal(browserResponseData)

	browserConnectionsToSendData := make([]*common.BrowserConnection, 0)
	browserConnectionsMutex.RLock()
	for _, bc := range browserConnections {
		if bc.MeetingId == meetingId {
			browserConnectionsToSendData = append(browserConnectionsToSendData, bc)
		}
	}
	browserConnectionsMutex.RUnlock()

	for _, bc := range browserConnectionsToSendData {
		bc.ActiveStreamingsMutex.RLock()
		queryIds, existsUserMediaGroupStream := bc.ActiveStreamings["getUserMediaGroupStateStream"]
		bc.ActiveStreamingsMutex.RUnlock()
		if existsUserMediaGroupStream {
			for i := range queryIds {
				payload := bytes.Replace(jsonDataNext, QueryIdPlaceholderInBytes, []byte(queryIds[i]), 1)
				bc.FromHasuraToBrowserChannel.TrySend(payload)
			}
		}
	}
}

// Cache key: meetingId -> "userId|groupId" -> row. A user can belong to
// multiple groups simultaneously so we cannot key by userId alone.
var (
	UserMediaGroupStatesCache      = make(map[string]map[string]map[string]any)
	UserMediaGroupStatesCacheMutex sync.RWMutex
)

func userMediaGroupCacheKey(userId string, groupId string) string {
	return userId + "|" + groupId
}

func GetUserMediaGroupStatesCache(meetingId string) (map[string]map[string]any, bool) {
	UserMediaGroupStatesCacheMutex.RLock()
	defer UserMediaGroupStatesCacheMutex.RUnlock()
	rows, ok := UserMediaGroupStatesCache[meetingId]
	if !ok {
		return nil, false
	}
	copyRows := make(map[string]map[string]any, len(rows))
	for key, row := range rows {
		newRow := make(map[string]any, len(row))
		maps.Copy(newRow, row)
		copyRows[key] = newRow
	}

	return copyRows, true
}

func StoreUserMediaGroupStatesCache(meetingId string, userId string, groupId string, row map[string]any) {
	UserMediaGroupStatesCacheMutex.Lock()
	defer UserMediaGroupStatesCacheMutex.Unlock()

	if _, exists := UserMediaGroupStatesCache[meetingId]; !exists {
		UserMediaGroupStatesCache[meetingId] = make(map[string]map[string]any)
	}
	UserMediaGroupStatesCache[meetingId][userMediaGroupCacheKey(userId, groupId)] = row
}

func RemoveMeetingUserMediaGroupStatesCache(meetingId string) {
	UserMediaGroupStatesCacheMutex.Lock()
	defer UserMediaGroupStatesCacheMutex.Unlock()
	delete(UserMediaGroupStatesCache, meetingId)
}

func RemoveUserMediaGroupStatesCache(meetingId string, userId string, groupId string) {
	UserMediaGroupStatesCacheMutex.Lock()
	defer UserMediaGroupStatesCacheMutex.Unlock()

	if rows, exists := UserMediaGroupStatesCache[meetingId]; exists {
		delete(rows, userMediaGroupCacheKey(userId, groupId))
		if len(rows) == 0 {
			delete(UserMediaGroupStatesCache, meetingId)
		}
	}
}

// Returns and clears every cached row for the given user. Returned maps are
// the original ones, callers should treat them as read-only.
func PopUserMediaGroupStatesCacheForUser(meetingId string, userId string) []map[string]any {
	UserMediaGroupStatesCacheMutex.Lock()
	defer UserMediaGroupStatesCacheMutex.Unlock()

	rows, exists := UserMediaGroupStatesCache[meetingId]
	if !exists {
		return nil
	}

	popped := make([]map[string]any, 0)
	for key, row := range rows {
		if row["userId"] == userId {
			popped = append(popped, row)
			delete(rows, key)
		}
	}
	if len(rows) == 0 {
		delete(UserMediaGroupStatesCache, meetingId)
	}
	return popped
}
