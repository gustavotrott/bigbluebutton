package replayer

import (
	"github.com/iMDT/bbb-graphql-middleware/internal/common"
	log "github.com/sirupsen/logrus"
)

func ReplaySubscriptionStartMessages(hc *common.HasuraConnection, fromBrowserToHasuraChannel *common.SafeChannel) {
	log := log.WithField("_routine", "ReplaySubscriptionStartMessages").WithField("browserConnectionId", hc.Browserconn.Id).WithField("hasuraConnectionId", hc.Id)

	hc.Browserconn.ActiveSubscriptionsMutex.RLock()
	for _, subscription := range hc.Browserconn.ActiveSubscriptions {
		if subscription.LastSeenOnHasuraConnetion != hc.Id {
			log.Tracef("replaying subscription start: %v", subscription.Message)
			fromBrowserToHasuraChannel.Send(subscription.Message)
		}
	}
	hc.Browserconn.ActiveSubscriptionsMutex.RUnlock()
}
