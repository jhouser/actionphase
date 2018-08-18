import React, {Component} from 'react';
import './index.scss';
import {Row, Col} from 'reactstrap';
import StatusControls from "../StatusControls";

class GameHeader extends Component {
    render() {
        return <div className="gameHeader">
            <Row>
                <Col sm="12">
                    <h2>{this.props.name} ({this.props.abbreviation})</h2>
                </Col>
            </Row>

            <Row>
                <Col sm="12">
                    <div className="gameHeader__attribute gameHeader__game_master">
                        by {this.props.game_master ? this.props.game_master.username : ''}
                    </div>
                </Col>
            </Row>

            <Row>
                <Col sm="12" md="1">
                    <div
                        className={`gameHeader__attribute gameHeader__status gameHeader__status-${this.props.get_status_display}`}>
                        {this.props.get_status_display}
                    </div>
                </Col>
                <Col sm="12" md="2">
                    <StatusControls status={this.props.get_status_display}/>
                </Col>
            </Row>
        </div>
    }
}

export default GameHeader;