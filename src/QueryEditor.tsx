import { defaults } from 'lodash';
import React from 'react';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { Select, InlineField, Label } from '@grafana/ui';

import { DataSource } from './DataSource';
import { graphs, MyQuery, MyDataSourceOptions, defaultQuery, maxInt32 } from './types';

interface Props extends QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions> {}

export function QueryEditor(props: Props) {
  const { query, onChange, onRunQuery } = defaults(props, defaultQuery);

  const onSelectGraph = (graph: SelectableValue<string>) => {
    onChange({
      ...query,
      graph: graph,
      pid: 0,
    });

    onRunQuery();
  }

  return (
    <div className="gf-form-inline">
      <InlineField 
        label="Graph:"
        className="gf-form-label width-14"
      >
        <Select
          className="gf-form-label width-10"
          placeholder={"Choose Graph"}
          options={graphs}
          value={query.graph}
          onChange={onSelectGraph}
        />
      </InlineField>
      <div className="gf-form" hidden={(query.pid == null || query.pid <= 0 || query.pid >= maxInt32)}>
        <Label className="gf-form-label width-10">
          &nbsp;PID:&nbsp;&nbsp;{query.pid}
        </Label>
      </div>
    </div>
  );
}
