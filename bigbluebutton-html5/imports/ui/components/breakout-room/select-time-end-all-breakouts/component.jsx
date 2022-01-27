import React, { useState } from 'react'
import { EndButton, SelectTimeEnd } from './styles'
import { defineMessages, injectIntl } from 'react-intl';

const intlMessages = defineMessages({
    seconds: {
      id: 'app.select-time-end-breakout.seconds',
      description: 'Option in seconds',
    },

    immediatly: {
      id: 'app.select-time-end-breakout.immediatly',
      description: 'Option immediatly',
    },
    endAllBreakouts: {
        id: 'app.createBreakoutRoom.endAllBreakouts',
        description: 'Button label to end all breakout rooms',
    },
})


export default injectIntl(function DropdownTimeEnd({ intl, endAllBreakouts, isMeteorConnected, closePanel }){

    const [secondsEndBreakout, setSecondsEndBreakout] = useState(30);

    return (
        <>
            <SelectTimeEnd
                    value={secondsEndBreakout} 
                    onChange={(e)=>setSecondsEndBreakout(e.target.value)}
            >
                <option value="60">{`60 ${intl.formatMessage(intlMessages.seconds)}`}</option>
                <option value="30">{`30 ${intl.formatMessage(intlMessages.seconds)}`}</option>
                <option value="0">{intl.formatMessage(intlMessages.immediatly)}</option>
            </SelectTimeEnd>
            <EndButton
                color="primary"
                disabled={!isMeteorConnected}
                size="lg"
                label={intl.formatMessage(intlMessages.endAllBreakouts)} 
                data-test="endBreakoutRoomsButton"
                onClick={() => {  

                    closePanel()                            
                    endAllBreakouts(secondsEndBreakout)
                }}
            />

        </>
    )
})

