import { DataQuery, DataSourceJsonData } from '@grafana/data';

export const graphProcesses = 'processes';

export const maxInt32: number = 2**31-1;

export interface MyQuery extends DataQuery {
  graph?: string;
  pid: number;
  streaming: boolean;
}

export const defaultQuery: MyQuery = {
  refId: '',
  graph: graphProcesses,
  pid: 0,
  streaming: false,
};

/**
 * These are options configured for each DataSource instance.
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
}

export const defaultDataSourceOptions: Partial<MyDataSourceOptions> = {
};
