import React from "react";
import {mount} from "enzyme";
import TextInput from ".";

describe("TextInput", () => {

    let props;
    let mountedTextInput;
    const textInput = () => {
        if (!mountedTextInput) {
            mountedTextInput = mount(
                <TextInput {...props} />
            );
        }
        return mountedTextInput;
    };

    beforeEach(() => {
        props = {
            name: 'test',
            label: undefined,
            error: undefined,
            type: undefined
        };
        mountedTextInput = undefined;
    });

    it("always renders a FormGroup", () => {
        const formGroup = textInput().find("FormGroup");
        expect(formGroup.length).toBe(1);
    });
    describe("the rendered FormGroup", () => {
        it("contains everything else that gets rendered", () => {
            const formGroup = textInput().find("FormGroup").first();
            expect(formGroup.children().length).toBeGreaterThan(0);
        });
    });
    it("always renders an Input", () => {
        expect(textInput().find("Input").length).toBe(1);
    });
    describe("the rendered Input", () => {
        it("receives props from TextInput", () => {
            const input = textInput().find("Input");
            expect(Object.keys(input.props()).length).toBeGreaterThan(0);
        });
        it("has a default type of 'text'", () => {
            const input = textInput().find("Input");
            expect(input.prop('type')).toBe("text");
        });
        it("has the 'name' property matching the prop", () => {
            const input = textInput().find("Input");
            expect(input.prop('name')).toBe(props.name);
        });
        it("has the 'id' property set to to 'id_$name'", () => {
            const input = textInput().find("Input");
            expect(input.prop('id')).toBe('id_' + props.name);
        });
    });
    describe("when 'label' is undefined", () => {
        it('does not render a Label', () => {
            const label = textInput().find("Label");
            expect(label.length).toBe(0);
        });
    });
    describe("when 'label' is defined", () => {
        beforeEach(() => {
            props.label = 'test';
        });
        it('causes a Label to be rendered', () => {
            const label = textInput().find("Label");
            expect(label.length).toBeGreaterThan(0);
        });
    });
});

