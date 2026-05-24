import { gql } from '@apollo/client';

// This subscription is handled by bbb-graphql-middleware (it never reaches
// Hasura). Items are pushed in response to UserMediaGroupStateEvtMsg events
// from akka-bbb-apps, with the middleware maintaining the snapshot of the
// current state per meeting. Do not change the operation name or selection
// shape without updating the middleware accordingly.
export const USER_MEDIA_GROUP_STATE_STREAM = gql`
  subscription getUserMediaGroupStateStream {
    user_mediaGroup_stream(
      cursor: { initial_value: { updatedAt: "2020-01-01" } },
      batch_size: 50
    ) {
      userId
      groupId
      mediaType
      sender
      receiver
      active
      removed
    }
  }
`;

export interface UserMediaGroupStateStreamResponse {
  user_mediaGroup_stream: Array<{
    userId: string;
    groupId: string;
    mediaType: string;
    sender: boolean;
    receiver: boolean;
    active: boolean;
    removed: boolean;
  }>;
}
