import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface MyQuery extends DataQuery {
  connected: boolean;
  kernel?: boolean;
  daemons?: boolean;
  files?: boolean;
}

export const defaultQuery: Partial<MyQuery> = {
  // "connected" is always true, a kludge to ensure a query is run even if other flags are false
  connected: true,
};

/**
 * These are options configured for each DataSource instance
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
 * Values used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  user?: string;
  password?: string;
}
