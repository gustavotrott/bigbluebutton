
import { makeCall } from '/imports/ui/services/api';

export const endAllBreakouts = (timeToEnd) => {
    makeCall('endAllBreakouts', timeToEnd);
};