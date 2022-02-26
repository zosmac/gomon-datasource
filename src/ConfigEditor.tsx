import defaults from 'lodash/defaults';
import React, { ChangeEvent, PureComponent } from 'react';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { LegacyForms } from '@grafana/ui';

import { defaultDataSourceOptions, MyDataSourceOptions, MySecureJsonData } from './types';

const { FormField, SecretFormField } = LegacyForms;

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions> {}

export class ConfigEditor extends PureComponent<Props> {
  onHostChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      host: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  onPathChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    const jsonData = {
      ...options.jsonData,
      path: event.target.value,
    };
    onOptionsChange({ ...options, jsonData });
  };

  // Secure field (only sent to the backend)
  onUserChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        user: event.target.value,
      },
    });
  };

  // Secure field (only sent to the backend)
  onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        password: event.target.value,
      },
    });
  };

  onResetUser = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        user: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        user: '',
      },
    });
  };

  onResetPassword = () => {
    const { onOptionsChange, options } = this.props;
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        password: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        password: '',
      },
    });
  };

  render() {
    const { options } = this.props;
    const { jsonData, secureJsonFields } = options;
    const { host, path } = defaults(jsonData, defaultDataSourceOptions);
    const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;

    return (
      <div className="gf-form-group">
        <div className="gf-form">
          <FormField
            label="Host"
            placeholder="http://localhost:1234"
            value={host}
            labelWidth={6}
            inputWidth={20}
            onChange={this.onHostChange}
          />
        </div>
        <div className="gf-form">
          <FormField
            label="Path"
            placeholder="/gomon"
            value={path}
            labelWidth={6}
            inputWidth={20}
            onChange={this.onPathChange}
          />
        </div>

        <div className="gf-form">
          <SecretFormField
            isConfigured={(secureJsonFields && secureJsonFields.user) as boolean}
            label="User"
            placeholder="Gomon user"
            labelWidth={6}
            inputWidth={20}
            value={secureJsonData.user || ''}
            onChange={this.onUserChange}
            onReset={this.onResetUser}
          />
        </div>
        <div className="gf-form-inline">
          <div className="gf-form">
            <SecretFormField
              isConfigured={(secureJsonFields && secureJsonFields.password) as boolean}
              label="Password"
              placeholder="Gomon user password"
              labelWidth={6}
              inputWidth={20}
              value={secureJsonData.password || ''}
              onChange={this.onPasswordChange}
              onReset={this.onResetPassword}
            />
          </div>
        </div>
      </div>
    );
  }
}
