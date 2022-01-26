import { DataSourceJsonData } from '@grafana/data';

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
