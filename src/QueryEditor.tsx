import { defaults } from 'lodash';
import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { MyQuery, MyDataSourceOptions, defaultQuery } from './types';

export type Properties = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor(props: Properties) {
  return <></>;
}

export function ExploreQueryEditor(props: Properties) {
  const { query } = props;
  const { node } = defaults(query, defaultQuery);
  return (
    <>
      <span hidden={node === ''}>
        <b>Node:</b> {node}
      </span>
    </>
  );
}
