import { useEffect, useMemo } from 'react';
import { makeVar, useReactiveVar, ObservableSubscription } from '@apollo/client';
import apolloContextHolder from '/imports/ui/core/graphql/apolloContextHolder/apolloContextHolder';
import logger from '/imports/startup/client/logger';
import {
  USER_MEDIA_GROUP_STATE_STREAM,
  UserMediaGroupStateStreamResponse,
} from './queries';
import { MediaGroupStream, MediaType } from './types';

type StateMap = Map<string, MediaGroupStream>;

const stateMapVar = makeVar<StateMap>(new Map());
const loadingVar = makeVar<boolean>(true);
const errorVar = makeVar<Error | null>(null);

let activeSubscription: ObservableSubscription | null = null;
let consumerCount = 0;

const keyFor = (userId: string, groupId: string) => `${userId}|${groupId}`;

const resetState = () => {
  stateMapVar(new Map());
  loadingVar(true);
  errorVar(null);
};

const startSubscriptionIfNeeded = () => {
  consumerCount += 1;
  if (activeSubscription) return;

  resetState();

  const apolloClient = apolloContextHolder.getClient();
  activeSubscription = apolloClient
    .subscribe<UserMediaGroupStateStreamResponse>({
      query: USER_MEDIA_GROUP_STATE_STREAM,
      fetchPolicy: 'no-cache',
    })
    .subscribe({
      next: ({ data }) => {
        if (!data) return;
        const entries = data.user_mediaGroup_stream || [];
        const next: StateMap = new Map(stateMapVar());
        let changed = false;

        entries.forEach((entry) => {
          const key = keyFor(entry.userId, entry.groupId);
          if (entry.removed) {
            if (next.delete(key)) changed = true;
          } else {
            next.set(key, {
              userId: entry.userId,
              groupId: entry.groupId,
              mediaType: entry.mediaType as MediaType,
              sender: entry.sender,
              receiver: entry.receiver,
              active: entry.active,
            });
            changed = true;
          }
        });

        if (changed) stateMapVar(next);
        if (loadingVar()) loadingVar(false);
      },
      error: (error) => {
        errorVar(error);
        loadingVar(false);
        logger.error({
          logCode: 'media_group_state_stream_error',
          extraInfo: { errorMessage: error.message },
        }, 'User media group state stream subscription failed.');
      },
    });
};

const stopSubscriptionIfNeeded = () => {
  consumerCount -= 1;
  if (consumerCount > 0) return;
  activeSubscription?.unsubscribe();
  activeSubscription = null;
  resetState();
};

export const useUserMediaGroupStateStream = (skip = false) => {
  useEffect(() => {
    if (skip) return undefined;
    startSubscriptionIfNeeded();
    return () => stopSubscriptionIfNeeded();
  }, [skip]);

  const stateMap = useReactiveVar(stateMapVar);
  const loading = useReactiveVar(loadingVar);
  const error = useReactiveVar(errorVar);

  const data = useMemo(() => Array.from(stateMap.values()), [stateMap]);

  return { data, loading, error };
};

export default useUserMediaGroupStateStream;
