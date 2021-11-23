import defaults from 'lodash/defaults';

import React, { ChangeEvent, PureComponent } from 'react';
import { Checkbox } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './datasource';
import { defaultQuery, MyDataSourceOptions, MyQuery } from './types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export class QueryEditor extends PureComponent<Props> {
  // "connected" is always true, a kludge to ensure a query is run even if other flags are false
  onConnectedChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, connected: event.target.checked });
  };

  onKernelChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, kernel: event.target.checked });
  };

  onDaemonsChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, daemons: event.target.checked });
  };

  onSyslogChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, syslog: event.target.checked });
  };

  onFilesChange = (event: ChangeEvent<HTMLInputElement>) => {
    const { onChange, query } = this.props;
    onChange({ ...query, files: event.target.checked });
  };

  render() {
    const query = defaults(this.props.query, defaultQuery);
    const { kernel, daemons, syslog, files } = query;

    return (
      <div className="gf-form">
        <Checkbox value={kernel} onChange={this.onKernelChange} label="Kernel" />
        <Checkbox value={daemons} onChange={this.onDaemonsChange} label="Daemons" />
        <Checkbox value={syslog} onChange={this.onSyslogChange} label="Syslog" />
        <Checkbox value={files} onChange={this.onFilesChange} label="Files" />
      </div>
    );
  }
}
