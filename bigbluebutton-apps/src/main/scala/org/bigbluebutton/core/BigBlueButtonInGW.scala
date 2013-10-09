package org.bigbluebutton.core

import org.bigbluebutton.core.api.ChangeUserStatus
import org.bigbluebutton.core.api.UserLeaving
import org.bigbluebutton.core.api.UserJoining
import org.bigbluebutton.core.api.UserJoining
import org.bigbluebutton.core.api.GetUsers
import org.bigbluebutton.core.api.AssignPresenter
import org.bigbluebutton.core.api.Role._
import org.bigbluebutton.core.api.IBigBlueButtonInGW
import org.bigbluebutton.core.api.CreateMeeting
import org.bigbluebutton.core.api.ClearPresentation
import org.bigbluebutton.core.api.SendCursorUpdate
import org.bigbluebutton.core.api.PresentationConversionUpdate
import org.bigbluebutton.core.api.RemovePresentation
import org.bigbluebutton.core.api.GetPresentationInfo
import org.bigbluebutton.core.api.ResizeAndMoveSlide
import org.bigbluebutton.core.api.GotoSlide
import org.bigbluebutton.core.api.SharePresentation
import org.bigbluebutton.core.api.GetSlideInfo
import org.bigbluebutton.conference.service.presentation.PreuploadedPresentationsUtil
import org.bigbluebutton.core.api.DestroyMeeting
import org.bigbluebutton.core.api.KeepAliveMessage
import org.bigbluebutton.core.api.PreuploadedPresentations
import scala.collection.JavaConversions._
import org.bigbluebutton.core.apps.poll.PollInGateway
import org.bigbluebutton.core.apps.layout.LayoutInGateway
import org.bigbluebutton.core.apps.chat.ChatInGateway
import scala.collection.JavaConversions._
import org.bigbluebutton.core.apps.whiteboard.WhiteboardInGateway
import org.bigbluebutton.core.apps.voice.VoiceInGateway

class BigBlueButtonInGW(bbbGW: BigBlueButtonGateway) extends IBigBlueButtonInGW {

  val presUtil = new PreuploadedPresentationsUtil()
    
  // Meeting
  def createMeeting2(meetingID: String, record: Boolean, voiceBridge: String) {
	bbbGW.accept(new CreateMeeting(meetingID, record, voiceBridge))
		
	val pres = presUtil.getPreuploadedPresentations(meetingID);
	if (!pres.isEmpty()) {
	  bbbGW.accept(new PreuploadedPresentations(meetingID, pres.toArray()))
	}
  }
  
  def destroyMeeting(meetingID: String) {
    bbbGW.accept(new DestroyMeeting(meetingID))
  }
  
  def isAliveAudit(aliveId:String) {
    bbbGW.acceptKeepAlive(new KeepAliveMessage(aliveId)); 
  }

  def statusMeetingAudit(meetingID: String) {
    
  }
	
  def endMeeting(meetingID: String) {
    
  }
	
  def endAllMeetings() {
    
  }
  
  /*************************************************************
   * Message Interface for Users
   *************************************************************/
  
	def setUserStatus(meetingID: String, userID: String, status: String, value: Object):Unit = {
		bbbGW.accept(new ChangeUserStatus(meetingID, userID, status, value));
	}

	def getUsers(meetingID: String, requesterID: String):Unit = {
		bbbGW.accept(new GetUsers(meetingID, requesterID))
	}

	def userLeft(meetingID: String, userID: String):Unit = {
		bbbGW.accept(new UserLeaving(meetingID, userID))
	}

	def userJoin(meetingID: String, userID: String, name: String, role: String, extUserID: String):Unit = {
		var userRole:Role = VIEWER;

		if (role == "MODERATOR") {
		  userRole = MODERATOR;
		}

		bbbGW.accept(new UserJoining(meetingID, userID, name, userRole, extUserID))
	}

	def assignPresenter(meetingID: String, newPresenterID: String, newPresenterName: String, assignedBy: String):Unit = {
		bbbGW.accept(new AssignPresenter(meetingID, newPresenterID, newPresenterName, assignedBy))
	}

	def getCurrentPresenter(meetingID: String, requesterID: String):Unit = {
		// do nothing
	}
	
	/**************************************************************************************
	 * Message Interface for Presentation
	 **************************************************************************************/

	def clear(meetingID: String) {
	  bbbGW.accept(new ClearPresentation(meetingID))
	}
	
	def sendUpdateMessage(meetingID: String, message: java.util.Map[String, Object]) {
	  bbbGW.accept(new PresentationConversionUpdate(meetingID, message))
	}
	
	def removePresentation(meetingID: String, presentationID: String) {
	  bbbGW.accept(new RemovePresentation(meetingID, presentationID))
	}
	
	def getPresentationInfo(meetingID: String, requesterID: String) {
	  bbbGW.accept(new GetPresentationInfo(meetingID, requesterID))
	}
	
	def sendCursorUpdate(meetingID: String, xPercent: Double, yPercent: Double) {
	  bbbGW.accept(new SendCursorUpdate(meetingID, xPercent, yPercent))
	}
	
	def resizeAndMoveSlide(meetingID: String, xOffset: Double, yOffset: Double, widthRatio: Double, heightRatio: Double) {
	  bbbGW.accept(new ResizeAndMoveSlide(meetingID, xOffset, yOffset, widthRatio, heightRatio))
	}
	
	def gotoSlide(meetingID: String, slide: Int) {
	  bbbGW.accept(new GotoSlide(meetingID, slide))
	}
	
	def sharePresentation(meetingID: String, presentationID: String, share: Boolean) {
	  bbbGW.accept(new SharePresentation(meetingID, presentationID, share))
	}
	
	def getSlideInfo(meetingID: String, requesterID: String) {
	  bbbGW.accept(new GetSlideInfo(meetingID, requesterID))
	}
	
	/**************************************************************
	 * Message Interface Polling
	 **************************************************************/
	val pollGW = new PollInGateway(bbbGW)
	
	def getPolls(meetingID: String, requesterID: String) {
	  pollGW.getPolls(meetingID, requesterID)
	}

	def preCreatedPoll(meetingID: String, msg: String) {
	  pollGW.preCreatedPoll(meetingID, msg)
	}
		
	def createPoll(meetingID: String, requesterID: String, msg: String) {
	  pollGW.createPoll(meetingID, requesterID, msg)
	}
	
	def updatePoll(meetingID: String, requesterID: String, msg: String) {
	  pollGW.updatePoll(meetingID, requesterID, msg)
	}
	
	def startPoll(meetingID: String, requesterID: String, msg: String) {
	  pollGW.startPoll(meetingID, requesterID, msg)
	}
	
	def stopPoll(meetingID: String, requesterID: String, msg: String) {
	  pollGW.stopPoll(meetingID, requesterID, msg)
	}
	
	def removePoll(meetingID: String, requesterID: String, msg: String) {
	  pollGW.removePoll(meetingID, requesterID, msg)
	}
	
	def respondPoll(meetingID: String, requesterID: String, msg: String) {
	  pollGW.respondPoll(meetingID, requesterID, msg)
	}
	
	def showPollResult(meetingID: String, requesterID: String, msg: String) {
	  pollGW.showPollResult(meetingID, requesterID, msg)
	}
	
	def hidePollResult(meetingID: String, requesterID: String, msg: String) {
	  pollGW.hidePollResult(meetingID, requesterID, msg)
	}
	
	/*************************************************************************
	 * Message Interface for Layout
	 *********************************************************************/
	val layoutGW = new LayoutInGateway(bbbGW)
	
	def getCurrentLayout(meetingID: String, requesterID: String) {
	  layoutGW.getCurrentLayout(meetingID, requesterID)
	}
	
	def setLayout(meetingID: String, requesterID: String, layoutID: String) {
	  layoutGW.setLayout(meetingID, requesterID, layoutID)
	}
	
	def lockLayout(meetingID: String, requesterID: String, layoutID: String) {
	  layoutGW.lockLayout(meetingID, requesterID, layoutID)
	}
	
	def unlockLayout(meetingID: String, requesterID: String) {
	  layoutGW.unlockLayout(meetingID, requesterID)
	}
	
	/*********************************************************************
	 * Message Interface for Chat
	 *******************************************************************/
	val chatGW = new ChatInGateway(bbbGW)
	
	def getChatHistory(meetingID: String, requesterID: String) {
	  chatGW.getChatHistory(meetingID, requesterID)
	}
	
	def sendPublicMessage(meetingID: String, requesterID: String, message: java.util.Map[String, String]) {
	  // Convert java Map to Scala Map, then convert Mutable map to immutable map
	  chatGW.sendPublicMessage(meetingID, requesterID, mapAsScalaMap(message).toMap)
	}
	
	def sendPrivateMessage(meetingID: String, requesterID: String, message: java.util.Map[String, String]) {
	  chatGW.sendPrivateMessage(meetingID, requesterID, mapAsScalaMap(message).toMap)
	}
	
	/*********************************************************************
	 * Message Interface for Whiteboard
	 *******************************************************************/
	val wbGW = new WhiteboardInGateway(bbbGW)
	
	def sendWhiteboardAnnotation(meetingID: String, requesterID: String, annotation: java.util.Map[String, Object]) {
	  wbGW.sendWhiteboardAnnotation(meetingID, requesterID, mapAsScalaMap(annotation).toMap)
	}
	
	def setWhiteboardActivePage(meetingID: String, requesterID: String, page: java.lang.Integer){
	  wbGW.setWhiteboardActivePage(meetingID, requesterID, page)
	}
	
	def requestWhiteboardAnnotationHistory(meetingID: String, requestedID: String, presentationID: String, page: java.lang.Integer) {
	  wbGW.requestWhiteboardAnnotationHistory(meetingID, requestedID, presentationID, page)
	}
	
	def clearWhiteboard(meetingID: String, requestedID: String) {
	  wbGW.clearWhiteboard(meetingID, requestedID);
	}
	
	def undoWhiteboard(meetingID: String, requestedID: String) {
	  wbGW.undoWhiteboard(meetingID, requestedID)
	}
	
	def setActivePresentation(meetingID: String, requestedID: String, presentationID: String, numPages: java.lang.Integer) {
	  wbGW.setActivePresentation(meetingID, requestedID, presentationID, numPages)
	}
	
	def enableWhiteboard(meetingID: String, requestedID: String, enable: java.lang.Boolean) {
	  wbGW.enableWhiteboard(meetingID, requestedID, enable)
	}
	
	def isWhiteboardEnabled(meetingID: String, requestedID: String) {
	  wbGW.isWhiteboardEnabled(meetingID, requestedID)
	}
	
	/*********************************************************************
	 * Message Interface for Voice
	 *******************************************************************/
	val voiceGW = new VoiceInGateway(bbbGW)
	
	def getVoiceUsers(meetingID: String, requesterID: String) {
	  voiceGW.getVoiceUsers(meetingID, requesterID)
	}
	
	def muteAllUsers(meetingID: String, requesterID: String, mute: java.lang.Boolean) {
	  voiceGW.muteAllUsers(meetingID, requesterID, mute)
	}
	
	def isMeetingMuted(meetingID: String, requesterID: String) {
	  voiceGW.isMeetingMuted(meetingID, requesterID)
	}
	
	def muteUser(meetingID: String, requesterID: String, userID: java.lang.Integer, mute: java.lang.Boolean) {
	  voiceGW.muteUser(meetingID, requesterID, userID, mute)
	}
	
	def lockUser(meetingID: String, requesterID: String, userID: java.lang.Integer, lock: java.lang.Boolean) {
	  voiceGW.lockUser(meetingID, requesterID, userID, lock)
	}
	
	def ejectUser(meetingID: String, requesterID: String, userID: java.lang.Integer) {
	  voiceGW.ejectUser(meetingID, requesterID, userID)
	}

	def voiceUserJoined(user: java.lang.Integer, voiceConfId: String, callerIdNum: String, callerIdName: String,
			muted: java.lang.Boolean, speaking: java.lang.Boolean) {
	  voiceGW.voiceUserJoined(user, voiceConfId, callerIdNum, callerIdName, muted, speaking)
	}
	
	def voiceUserLeft(user: java.lang.Integer, voiceConfId: String) {
	  voiceGW.voiceUserLeft(user, voiceConfId)
	}
	
	def voiceUserMuted(user: java.lang.Integer, voiceConfId: String, muted: java.lang.Boolean) {
	  voiceGW.voiceUserMuted(user, voiceConfId, muted)
	}
	
	def voiceUserTalking(user: java.lang.Integer, voiceConfId: String, talking: java.lang.Boolean) {
	  voiceGW.voiceUserTalking(user, voiceConfId, talking)
	}
	
	def voiceStartedRecording(voiceConfId: String, filename: String, timestamp: String, record: java.lang.Boolean) {
	  voiceGW.voiceStartedRecording(voiceConfId, filename, timestamp, record)
	}
	
}