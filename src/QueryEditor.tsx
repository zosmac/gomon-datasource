import { defaults } from 'lodash';
import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { MyQuery, MyDataSourceOptions, defaultQuery } from './types';

export type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor(props: Props) {
  return <></>;
}

export function ExploreQueryEditor(props: Props) {
  const { query } = props;
  const { node } = defaults(query, defaultQuery);
  const { process, host, data } = node;
  return (
    <>
      <span hidden={process === undefined}>
        <b>Process ID:</b> {process}
      </span>
      <span hidden={host === undefined}>
        <b>IP Address:</b> {host}
      </span>
      <span hidden={data === undefined}>
        <b>Data:</b> {data}
      </span>
    </>
  );
}
