
import { withTracker } from 'meteor/react-meteor-data';
import DropdownTimeEndComponent from './component'
import { endAllBreakouts} from './service';


export default withTracker(()=>{
    const isMeteorConnected = Meteor.status().connected;
    return {

        isMeteorConnected,
        endAllBreakouts
    }
})(DropdownTimeEndComponent)