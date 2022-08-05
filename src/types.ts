import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface MyQuery extends DataQuery {
  query: string;
  streaming: boolean;
}

export const defaultQuery: MyQuery = {
  refId: '',
  query: '0',
  streaming: false,
};

/**
 * These are options configured for each DataSource instance.
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  host: string;
  path: string;
}

export const defaultDataSourceOptions: Partial<MyDataSourceOptions> = {
  host: 'http://localhost:1234',
  path: '/gomon',
};

/**
 * Values used in the backend, but never sent over HTTP to the frontend.
 */
export interface MySecureJsonData {
  user?: string;
  password?: string;
}
