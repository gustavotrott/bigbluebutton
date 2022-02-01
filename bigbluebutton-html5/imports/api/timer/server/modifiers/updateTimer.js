import { check } from 'meteor/check';
import Timer from '/imports/api/timer';
import Logger from '/imports/startup/server/logger';
import Users from '/imports/api/users';
import { TRACKS, getDefaultTime } from '/imports/api/timer/server/helpers';

const getActivateModifier = () => {
  const time = getDefaultTime();
  check(time, Number);

  return {
    $set: {
      stopwatch: true,
      active: true,
      running: false,
      time,
      accumulated: 0,
      timestamp: 0,
      track: TRACKS[0],
      ended: 0,
    },
  };
};

const getDeactivateModifier = () => {
  return {
    $set: {
      active: false,
      running: false,
      ended: 0,
    },
  };
};

const getResetModifier = () => {
  return {
    $set: {
      accumulated: 0,
      timestamp: Date.now(),
      ended: 0,
    },
  };
};

const handleTimerEndedNotifications = (fields, meetingId, handle) => {
  const meetingUsers = Users.find({
    meetingId,
    connectionStatus: 'online',
  }).count();

  if (fields.running === false) {
    handle.stop();
  }

  //Timer is stopped when at least 90% of users online in the meetinng notify that it ended.
  if (fields.ended >= Math.round(0.9 * meetingUsers)) {
    const accumulated = 0;
    updateTimer('stop', meetingId, 0, false, accumulated);
  }
};

const setTimerEndObserver = (meetingId) => {
  const { stopwatch } = Timer.findOne({ meetingId });

  if (stopwatch === false) {
    const meetingTimer = Timer.find(
      { meetingId },
      { fields: { ended: 1, running: 1 } },
    );
    const handle = meetingTimer.observeChanges({
      changed: (id, fields) => {
        handleTimerEndedNotifications(fields, meetingId, handle);
      },
    });
  }
};

const getStartModifier = () => {
  return {
    $set: {
      running: true,
      timestamp: Date.now(),
      ended: 0,
    },
  };
};

const getStopModifier = (accumulated) => {
  return {
    $set: {
      running: false,
      accumulated,
      timestamp: 0,
      ended: 0,
    },
  };
};

const getSwitchModifier = (stopwatch) => {
  return {
    $set: {
      stopwatch,
      running: false,
      accumulated: 0,
      timestamp: 0,
      track: TRACKS[0],
      ended: 0,
    },
  };
};

const getSetModifier = (time) => {
  return {
    $set: {
      running: false,
      accumulated: 0,
      timestamp: 0,
      time,
    },
  };
};

const getTrackModifier = (track) => {
  return {
    $set: {
      track,
    },
  };
};

const getEndedModifier = () => {
  return {
    $inc: {
      ended: 1,
    },
  };
};

export default function updateTimer(action, meetingId, time = 0, stopwatch = true, accumulated = 0, track = TRACKS[0]) {
  check(action, String);
  check(meetingId, String);
  check(time, Number);
  check(stopwatch, Boolean);
  check(accumulated, Number);
  check(track, String);

  const selector = {
    meetingId,
  };

  let modifier;

  switch(action) {
    case 'activate':
      modifier = getActivateModifier();
      break;
    case 'deactivate':
      modifier = getDeactivateModifier();
      break;
    case 'reset':
      modifier = getResetModifier();
      break;
    case 'start':
      setTimerEndObserver(meetingId);
      modifier = getStartModifier();
      break;
    case 'stop':
      modifier = getStopModifier(accumulated);
      break;
    case 'switch':
      modifier = getSwitchModifier(stopwatch);
      break;
    case 'set':
      modifier = getSetModifier(time);
      break;
    case 'track':
      modifier = getTrackModifier(track);
      break;
    case 'ended':
      modifier = getEndedModifier();
      break;
    default:
      Logger.error(`Unhandled timer action=${action}`);
  }

  const cb = (err) => {
    if (err) {
      return Logger.error(`Updating timer at collection: ${err}`);
    }

    return Logger.debug(`Updated timer action=${action} meetingId=${meetingId}`);
  };

  return Timer.update(selector, modifier, cb);
}
