import { DataQuery, DataSourcePlugin } from '@grafana/data';
import { DataSource } from './DataSource';
import { ConfigEditor } from './ConfigEditor';
import { MyDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, DataQuery, MyDataSourceOptions>(DataSource).setConfigEditor(
  ConfigEditor
);
