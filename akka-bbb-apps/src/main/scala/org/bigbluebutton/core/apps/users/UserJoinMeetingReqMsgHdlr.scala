package org.bigbluebutton.core.apps.users

import org.bigbluebutton.common2.msgs.UserJoinMeetingReqMsg
import org.bigbluebutton.core.apps.breakout.BreakoutHdlrHelpers
import org.bigbluebutton.core.domain.MeetingState2x
import org.bigbluebutton.core.models.{ RegisteredUsers, Users2x, VoiceUsers }
import org.bigbluebutton.core.running.{ HandlerHelpers, LiveMeeting, MeetingActor, OutMsgRouter }
import org.bigbluebutton.core2.message.senders.MsgBuilder

trait UserJoinMeetingReqMsgHdlr extends HandlerHelpers {
  this: MeetingActor =>

  val liveMeeting: LiveMeeting
  val outGW: OutMsgRouter

  def handleUserJoinMeetingReqMsg(msg: UserJoinMeetingReqMsg, state: MeetingState2x): MeetingState2x = {
    log.info("Received user joined meeting. user {} meetingId={}", msg.body.userId, msg.header.meetingId)

    Users2x.findWithIntId(liveMeeting.users2x, msg.body.userId) match {
      case Some(reconnectingUser) =>
        if (reconnectingUser.userLeftFlag.left) {
          log.info("Resetting flag that user left meeting. user {}", msg.body.userId)
          // User has reconnected. Just reset it's flag. ralam Oct 23, 2018
          sendUserLeftFlagUpdatedEvtMsg(outGW, liveMeeting, msg.body.userId, false)
          Users2x.resetUserLeftFlag(liveMeeting.users2x, msg.body.userId)
        }

        state
      case None =>
        val newState = userJoinMeeting(outGW, msg.body.authToken, msg.body.clientType, liveMeeting, state)

        if (liveMeeting.props.meetingProp.isBreakout) {
          BreakoutHdlrHelpers.updateParentMeetingWithUsers(liveMeeting, eventBus)
        }

        // fresh user joined (not due to reconnection). Clear (pop) the cached voice user
        VoiceUsers.recoverVoiceUser(liveMeeting.voiceUsers, msg.body.userId)

        // Warn previous users that someone connected with same Id
        for {
          regUser <- RegisteredUsers.getRegisteredUserWithToken(msg.body.authToken, msg.body.userId,
            liveMeeting.registeredUsers)
        } yield {
          RegisteredUsers.findAllWithExternUserId(regUser.externId, liveMeeting.registeredUsers)
            .filter(u => u.id != regUser.id)
            .foreach { previousUser =>
              val notifyUserEvent = MsgBuilder.buildNotifyUserInMeetingEvtMsg(
                previousUser.id,
                liveMeeting.props.meetingProp.intId,
                "info",
                "promote",
                "app.mobileAppModal.userConnectedWithSameId",
                "Notification to warn that user connect again from other browser/device",
                Vector(regUser.name)
              )
              outGW.send(notifyUserEvent)
            }
        }

        newState
    }
  }
}

