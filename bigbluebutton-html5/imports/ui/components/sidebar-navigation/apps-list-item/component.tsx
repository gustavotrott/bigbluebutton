import React, { memo, useEffect } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { PANELS } from '/imports/ui/components/layout/enums';
import { layoutSelectInput } from '/imports/ui/components/layout/context';
import { Input } from '/imports/ui/components/layout/layoutTypes';
import SidebarNavigationButton from '/imports/ui/components/sidebar-navigation/sidebar-navigation-button/component';
import usePanelClose from '/imports/ui/components/common/panel-header/usePanelClose';
import useIsSpecificPanelOpened from '../hooks/useIsSpecificPanelOpened';

const intlMessages = defineMessages({
  wigetsLabel: {
    id: 'app.userList.appsTitle',
    description: 'Title for the apps panel',
  },
});

const AppsListItem: React.FC = () => {
  const intl = useIntl();
  const isOpened = useIsSpecificPanelOpened(PANELS.APPS_GALLERY);
  const label = intl.formatMessage(intlMessages.wigetsLabel);
  // Apps are registered according to the user's role (e.g. polls only for the
  // presenter, timer only for moderators) and plugin availability, so an empty
  // list means there is nothing this user can pick from the gallery.
  const registeredApps = layoutSelectInput((i: Input) => i.sidebarNavigation.registeredApps);
  const hasApps = Object.keys(registeredApps || {}).length > 0;
  const { closePanel } = usePanelClose(PANELS.APPS_GALLERY);

  useEffect(() => {
    // Close the gallery if its last app went away while it was open
    // (e.g. the user lost presenter/moderator role).
    if (!hasApps && isOpened) closePanel();
  }, [hasApps, isOpened]);

  if (!hasApps) return null;

  return (
    <SidebarNavigationButton
      panel={PANELS.APPS_GALLERY}
      isOpened={isOpened}
      iconName="widgets"
      label={label}
      id="apps-gallery-toggle-button"
      ariaDescribedBy="appsGallery"
      dataTest="appsGallerySidebarButton"
    />
  );
};

export default memo(AppsListItem);
