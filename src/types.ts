import { DataQuery, DataSourceJsonData, SelectableValue } from '@grafana/data';

export const graphMetrics:   SelectableValue<string> = { label: 'metrics' };
export const graphLogs:      SelectableValue<string> = { label: 'logs' };
export const graphProcesses: SelectableValue<string> = { label: 'processes' };
export const graphs: Array<SelectableValue<string>> = [ graphMetrics, graphLogs, graphProcesses ];

export const levelTrace: SelectableValue<string> = { label: 'trace' };
export const levelDebug: SelectableValue<string> = { label: 'debug' };
export const levelInfo:  SelectableValue<string> = { label: 'info' };
export const levelWarn:  SelectableValue<string> = { label: 'warn' };
export const levelError: SelectableValue<string> = { label: 'error' };
export const levels: Array<SelectableValue<string>> = [ levelTrace, levelDebug, levelInfo, levelWarn, levelError ];

export const maxInt32: number = 2**31-1;

export interface MyQuery extends DataQuery {
  graph?: SelectableValue<string>
  pid: number;
  streaming: boolean;
}

export const defaultQuery: MyQuery = {
  refId: '',
  graph: graphMetrics,
  pid: 0,
  streaming: false,
};

/**
 * These are options configured for each DataSource instance.
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  host?: string;
  path?: string;
  level?: SelectableValue<string>;
}

export const defaultDataSourceOptions: Partial<MyDataSourceOptions> = {
  host: 'http://localhost:1234',
  path: '/gomon',
  level: levelInfo,
};

/**
 * Values used in the backend, but never sent over HTTP to the frontend.
 */
export interface MySecureJsonData {
  user?: string;
  password?: string;
}
