import defaults from 'lodash/defaults';
import React, { useState } from 'react';
import { DataSourcePluginOptionsEditorProps, SelectableValue } from '@grafana/data';
import { LegacyForms, Label, Select } from '@grafana/ui';

import { logLevels, defaultDataSourceOptions, MyDataSourceOptions, MySecureJsonData } from './types';

const { FormField, SecretFormField } = LegacyForms;

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions> {}

export function ConfigEditor(props: Props){
  const { options, onOptionsChange } = props;
  const { jsonData, secureJsonFields } = options;
  const { host, path, level } = defaults(jsonData, defaultDataSourceOptions);
  const [ getHost, setHost ] = useState(host);
  const [ getPath, setPath ] = useState(path);
  const [ getLevel, setLevel ] = useState(level);
  const secureJsonData = (options.secureJsonData || {}) as MySecureJsonData;

  const onHostChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setHost(event.target.value);
    onOptionsChange({
      ...options,
      jsonData: {
        host: event.target.value,
        path: getPath,
        level: getLevel,
      },
    });
  };

  const onPathChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setPath(event.target.value);
    onOptionsChange({
      ...options,
      jsonData: {
        host: getHost,
        path: event.target.value,
        level: getLevel,
      },
    });
  };

  // Secure field (only sent to the backend)
  const onUserChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        user: event.target.value,
      },
    });
  };

  // Secure field (only sent to the backend)
  const onPasswordChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        password: event.target.value,
      },
    });
  };

  const onResetUser = () => {
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

  const onResetPassword = () => {
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

  const onSelectLevel = (level: SelectableValue<string>) => {
    setLevel(level);
    onOptionsChange({
      ...options,
      jsonData: {
        host: getHost,
        path: getPath,
        level: level,
      },
    });
  };

  return (
    <div className="gf-form-group">
      <div className="gf-form">
        <FormField
          label="Host"
          placeholder="http://localhost:1234"
          value={getHost}
          labelWidth={5}
          inputWidth={12}
          onChange={onHostChange}
        />
      </div>

      <div className="gf-form">
        <FormField
          label="Path"
          placeholder="/gomon"
          value={getPath}
          labelWidth={5}
          inputWidth={12}
          onChange={onPathChange}
        />
      </div>

      <div className="gf-form">
        <SecretFormField
          isConfigured={(secureJsonFields && secureJsonFields.user) as boolean}
          label="User"
          placeholder="Gomon user"
          labelWidth={5}
          inputWidth={12}
          value={secureJsonData.user || ''}
          onChange={onUserChange}
          onReset={onResetUser}
        />
      </div>
      <div className="gf-form">
        <SecretFormField
          isConfigured={(secureJsonFields && secureJsonFields.password) as boolean}
          label="Password"
          placeholder="Gomon user password"
          labelWidth={5}
          inputWidth={12}
          value={secureJsonData.password || ''}
          onChange={onPasswordChange}
          onReset={onResetPassword}
        />
      </div>

      <div className="gf-form">
        <Label className="gf-form-label width-5">Log Level</Label>
        <Select
          width={24}
          options={logLevels}
          placeholder="Choose log level"
          value={getLevel}
          onChange={onSelectLevel}
        />
      </div>
    </div>
  );
}
