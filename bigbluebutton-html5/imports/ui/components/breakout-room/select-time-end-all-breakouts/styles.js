import styled from 'styled-components';
import {
    colorWhite,
    colorGray, 
    colorGrayLighter, 
} from '../../../stylesheets/styled-components/palette'

import Button from '../../button/component';
import { headingsFontWeight, fontSizeSmall } from '/imports/ui/stylesheets/styled-components/typography';

export const SelectTimeEnd = styled.select`
  background-color: ${colorWhite};
  color: ${colorGray};
  border: 1px solid ${colorGrayLighter};
  border-radius: 5px;
  width: 100%;
  padding-top: .25rem;
  padding-bottom: .25rem;
  padding: .25rem 0 .25rem .25rem;
  margin-bottom: 5px;
`;


export const EndButton = styled(Button)`
  width: 100%;
  padding: .5rem;
  font-weight: ${headingsFontWeight} !important;
  border-radius: .2rem;
  font-size: ${fontSizeSmall};
`;