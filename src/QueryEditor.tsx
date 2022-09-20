import { defaults } from 'lodash';
import React, { useState } from 'react';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { Label, Select, LegacyForms } from '@grafana/ui';

import { DataSource } from './DataSource';
import { graphs, MyQuery, MyDataSourceOptions, defaultQuery } from './types';

const { FormField } = LegacyForms;

interface Props extends QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions> {}

export function QueryEditor(props: Props) {
  const { query, onChange, onRunQuery } = defaults(props, defaultQuery);
  const [ state, setState ] = useState(query);

  const onSelectGraph = (graph: SelectableValue<string>) => {
    setState({
      ...state,
      graph: graph,
    });

    onChange({
      ...state,
      graph: graph,
    });

    onRunQuery();
  }

  const onPidChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setState({
      ...state,
      pid: Number(event.target.value),
    });
    onChange({
      ...state,
      pid: Number(event.target.value),
    });
    onRunQuery();
  }

  return (
    <div className="gf-form-inline">
      <div className="gf-form">
        <Label className="gf-form-label width-6">Select Graph</Label>
        <Select
          width={20}
          placeholder={"Choose Graph"}
          options={graphs}
          value={state.graph}
          onChange={onSelectGraph}
        />
      </div>
      <div className="gf-form">
        <FormField
          label="Pid"
          labelWidth={3}
          placeholder={"Pid"}
          inputWidth={8}
          readOnly={true}
          defaultValue={0}
          value={state.pid}
          onBlur={onPidChange}
          onChange={onPidChange}
        />
      </div>
    </div>
  );
}
