import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface MyQuery extends DataQuery {
  connected: boolean; // not sure why this needed, but enables "Run query" with first use of Explore
  node: {
    process?: string;
    host?: string;
    data?: string;
  };
  streaming: boolean;
}

export const defaultQuery: MyQuery = {
  connected: true, // set to prevent 'hide': true in query that disables "Run query"
  node: {},
  streaming: false,
  refId: '',
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
