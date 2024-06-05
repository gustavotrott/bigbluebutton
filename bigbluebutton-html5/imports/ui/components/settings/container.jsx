import React from 'react';
import Settings from './component';
import { layoutDispatch } from '../layout/context';
import { useIsScreenSharingEnabled } from '/imports/ui/services/features';
import UserReactionService from '/imports/ui/components/user-reaction/service';

import {
  updateSettings,
  getAvailableLocales,
} from './service';
import useUserChangedLocalSettings from '../../services/settings/hooks/useUserChangedLocalSettings';
import { useShouldRenderPaginationToggle } from '/imports/ui/components/video-provider/video-provider-graphql/hooks';
import useSettings from '/imports/ui/services/settings/hooks/useSettings';
import { SETTINGS } from '/imports/ui/services/settings/enums';
import useCurrentUser from '/imports/ui/core/hooks/useCurrentUser';
import useMeeting from '/imports/ui/core/hooks/useMeeting';

const ASK_MODERATOR = 'ASK_MODERATOR';

const SettingsContainer = (props) => {
  const layoutContextDispatch = layoutDispatch();
  const setLocalSettings = useUserChangedLocalSettings();
  const paginationToggleEnabled = useShouldRenderPaginationToggle();
  const { data: currentUser } = useCurrentUser((u) => ({
    presenter: u.presenter,
    isModerator: u.isModerator,
  }));
  const { data: meeting } = useMeeting((m) => ({
    usersPolicies: {
      guestPolicy: m.usersPolicies.guestPolicy,
    },
  }));
  const application = useSettings(SETTINGS.APPLICATION);
  const audio = useSettings(SETTINGS.AUDIO);
  const dataSaving = useSettings(SETTINGS.DATA_SAVING);
  const availableLocales = getAvailableLocales();
  const isPresenter = currentUser?.presenter ?? false;
  const isModerator = currentUser?.isModerator ?? false;
  const isScreenSharingEnabled = useIsScreenSharingEnabled();
  const showGuestNotification = meeting?.usersPolicies?.guestPolicy === ASK_MODERATOR;
  const isReactionsEnabled = UserReactionService.useIsEnabled();

  return (
    <Settings
      {...{
        ...props,
        updateSettings,
        application,
        audio,
        dataSaving,
        availableLocales,
        isPresenter,
        isModerator,
        isScreenSharingEnabled,
        showGuestNotification,
        isReactionsEnabled,
        showToggleLabel: false,
        isVideoEnabled: window.meetingClientSettings.public.kurento.enableVideo,
      }}
      layoutContextDispatch={layoutContextDispatch}
      setLocalSettings={setLocalSettings}
      paginationToggleEnabled={paginationToggleEnabled}
    />
  );
};

export default SettingsContainer;
