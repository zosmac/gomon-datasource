import { defaults } from 'lodash';
import React, { useState } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { MyQuery, MyDataSourceOptions, defaultQuery } from './types';

interface Props extends QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions> {}

export function QueryEditor(props: Props) {
  return ExploreQueryEditor(props);
}

export function ExploreQueryEditor(props: Props) {
  const { query, onChange, onRunQuery } = defaults(props, defaultQuery);
  const [ value, setValue ] = useState(query);

  const onSelect = (event: React.ChangeEvent<HTMLSelectElement>) => {
    setValue({...value,
      queryText: event.target.value,
    });

    onChange({...value,
      queryText: event.target.value,
    });

    onRunQuery();
  }

  return (
    <>
      Select a graph:
      <select name="graph" value={query.queryText} onChange={onSelect}>&nbsp;
      { ['metrics', 'logs', 'processes'].map((option: string) =>
        <option key={option} value={option}>{option}</option>
      )}
      </select>
    </>
  );
}
