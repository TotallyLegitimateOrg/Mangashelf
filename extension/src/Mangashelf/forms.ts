/* SPDX-License-Identifier: GPL-3.0-or-later */

import {
  ButtonRow,
  Form,
  InputRow,
  LabelRow,
  Section,
  type FormSectionElement,
  type SelectorID,
} from "@paperback/types";

class State<T> {
  private _value: T;
  public get value(): T {
    return this._value;
  }

  public get selector(): SelectorID<(value: T) => Promise<void>> {
    return Application.Selector(this as State<T>, "updateValue");
  }

  constructor(
    private form: Form,
    value: T,
    private onChange?: (value: T) => Promise<void>,
  ) {
    this._value = value;
  }

  public async updateValue(value: T): Promise<void> {
    this._value = value;
    await this.onChange?.(value);
    this.form.reloadForm();
  }

  public setValue(value: T): void {
    this._value = value;
  }
}

export class SettingsForm extends Form {
  serverUrl = new State<string>(this, "");
  apiKey = new State<string>(this, "");
  testButtonTitle = new State<string>(this, "Test Connection");
  connectionStatus = new State<string>(this, "Not tested yet.");
  constructor(
    private callbacks: {
      onServerUrlChange: (value: string) => Promise<void>;
      onApiKeyChange: (value: string) => Promise<void>;
      onTestConnection: () => Promise<void>;
    },
  ) {
    super();
    this.serverUrl = new State<string>(this, "", this.callbacks.onServerUrlChange);
    this.apiKey = new State<string>(this, "", this.callbacks.onApiKeyChange);
  }

  public setPersistedValues(values: { serverUrl?: string; apiKey?: string }): void {
    this.serverUrl.setValue(values.serverUrl ?? "");
    this.apiKey.setValue(values.apiKey ?? "");
  }

  public setConnectionStatus(message: string): void {
    this.connectionStatus.setValue(message);
  }

  public setTesting(isTesting: boolean): void {
    this.testButtonTitle.setValue(isTesting ? "Testing..." : "Test Connection");
  }

  override getSections(): FormSectionElement<unknown>[] {
    return [
      Section(
        {
          id: "server",
          footer:
            "Generate an API key from Settings in the web app, then enter your Mangashelf server URL and API key here.",
        },
        [
          InputRow("serverUrl", {
            title: "Server URL",
            value: this.serverUrl.value,
            onValueChange: this.serverUrl.selector,
          }),
          InputRow("apiKey", {
            title: "API Key",
            value: this.apiKey.value,
            isSecureEntry: true,
            onValueChange: this.apiKey.selector,
          }),
          ButtonRow("testConnection", {
            title: this.testButtonTitle.value,
            onSelect: Application.Selector(this.callbacks, "onTestConnection"),
          }),
          LabelRow("connectionStatus", {
            title: "Connection",
            subtitle: this.connectionStatus.value,
            value: "",
          }),
        ],
      ),
    ];
  }
}
