import { defaults } from 'lodash';
import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { Button, InlineField, Label } from '@grafana/ui';

import { DataSource } from './DataSource';
import { MyQuery, MyDataSourceOptions, defaultQuery, graphProcesses, maxInt32 } from './types';

interface Props extends QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions> {}

export function QueryEditor(props: Props) {
  const { query, onChange, onRunQuery } = defaults(props, defaultQuery);
  const onClickGraph = () => {
    onChange({
      ...query,
      graph: graphProcesses,
      pid: 0,
    });

    onRunQuery();
  }

  return (
    <div className="gf-form-inline">
      <InlineField
        label="NodeGraph:"
        className="gf-form-label width-14"
      >
        <Button
          className="gf-form-button"
          onClick={onClickGraph}
          >
          {graphProcesses}
        </Button>
      </InlineField>
      <div className="gf-form" hidden={(query.pid == null || query.pid <= 0 || query.pid >= maxInt32)}>
        <Label className="gf-form-label width-10">
          &nbsp;PID:&nbsp;&nbsp;{query.pid}
        </Label>
      </div>
    </div>
  );
}
