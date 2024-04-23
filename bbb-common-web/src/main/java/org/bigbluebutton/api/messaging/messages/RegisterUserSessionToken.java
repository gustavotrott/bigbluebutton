package org.bigbluebutton.api.messaging.messages;


import java.util.Map;

public class RegisterUserSessionToken implements IMessage {

	public final String meetingID;
	public final String internalUserId;
	public final String sessionToken;

	public RegisterUserSessionToken(String meetingID, String internalUserId, String sessionToken) {
		this.meetingID = meetingID;
		this.internalUserId = internalUserId;
		this.sessionToken = sessionToken;
	}
}
