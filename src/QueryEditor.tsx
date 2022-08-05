import { defaults } from 'lodash';
import React from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { MyQuery, MyDataSourceOptions, defaultQuery } from './types';

export type Properties = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor(props: Properties) {
//  return <></>;
  const { query } = defaults(props.query, defaultQuery);
  return (
    <>
      <span hidden={query === ''}>
        <b>Query:</b> {query}
      </span>
    </>
  );
}

export function ExploreQueryEditor(props: Properties) {
  const { query } = defaults(props.query, defaultQuery);
  return (
    <>
      <span hidden={query === ''}>
        <b>Query:</b> {query}
      </span>
    </>
  );
}
