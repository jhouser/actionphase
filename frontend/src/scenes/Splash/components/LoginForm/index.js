import React, {Component} from 'react';
import {Link} from "react-router-dom";
import PropTypes from 'prop-types'
import FormInput from '../../../../components/FormInput'
import {FormGroup, Alert, Button, Form} from 'reactstrap'
import './index.scss';

class LoginForm extends Component {
    state = {
        username: '',
        password: ''
    };
    handleInputChange = (event) => {
        const target = event.target,
            value = target.type ===
            'checkbox' ? target.checked : target.value,
            name = target.name;
        this.setState({
            [name]: value
        });
    };
    onSubmit = (event) => {
        event.preventDefault();
        this.props.onSubmit(this.state.username, this.state.password)
    };

    render() {
        const errors = this.props.errors || {};
        return <div className="login-form">
            <Form onSubmit={this.onSubmit}>
                {
                    errors.non_field_errors ?
                        <Alert color="danger">
                            {errors.non_field_errors}
                        </Alert> : ""
                }
                <FormInput className="login-form__item" name="username" placeholder="Username" error={errors.username}
                           onChange={this.handleInputChange}/>
                <FormInput className="login-form__item" name="password" placeholder="Password" error={errors.password}
                           type="password" onChange={this.handleInputChange}/>
                <Button type="submit" color="primary" className="login-form__submit-button">Login</Button>
                <FormGroup className="login-form__links">
                    <Link className="login-form__link" to="/home/register">Need an account?</Link><br/>
                    <Link className="login-form__link" to="/">Forgot your password?</Link>
                </FormGroup>
            </Form>
        </div>
    }
}

LoginForm.propTypes = {
    onSubmit: PropTypes.func.isRequired,
    errors: PropTypes.object
};

export default LoginForm;
