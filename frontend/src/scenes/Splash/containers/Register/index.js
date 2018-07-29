import React from 'react';
import {withRouter} from "react-router-dom";
import {connect} from 'react-redux';
import {register} from '../../../../actions/auth';
import {authErrors} from '../../../../util/auth';
import RegistrationForm from "../../components/RegistrationForm";
import {formApiAdapter} from "../../../../util/forms";

const Register = (props) => {
    return (
        <RegistrationForm {...props}/>
    )
};
/* istanbul ignore next */
const mapStateToProps = state => ({
    errors: authErrors(state)
});
/* istanbul ignore next */
const mapDispatchToProps = dispatch => ({
    onSubmit: () => {return formApiAdapter(dispatch, register);}
});
/* istanbul ignore next */
export default withRouter(connect(mapStateToProps, mapDispatchToProps)(Register));
