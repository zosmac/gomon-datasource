import { defaults } from 'lodash';
import React, { useState } from 'react';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from './DataSource';
import { MyQuery, MyDataSourceOptions, defaultQuery } from './types';

interface Props extends QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions> {}

export function QueryEditor(props: Props) {
  return ExploreQueryEditor(props);
}

export function ExploreQueryEditor(props: Props) {
  const { query, onChange, onRunQuery } = defaults(props, defaultQuery);
  const [ value, setValue ] = useState(query);

  const onSelect = (event: React.ChangeEvent<HTMLSelectElement>) => {
    setValue({...value,
      queryText: event.target.value,
      metrics: event.target.value === "metrics",
      logs: event.target.value === "logs",
      processes: event.target.value === "processes"
    });

    onChange({...value,
      queryText: event.target.value,
      metrics: event.target.value === "metrics",
      logs: event.target.value === "logs",
      processes: event.target.value === "processes"
    });

    onRunQuery();
  }

  // const onRadioClick = (event: React.ChangeEvent<HTMLInputElement>) => {
  //   setValue({...value,
  //     queryText: event.target.value,
  //     metrics: event.target.value === "metrics",
  //     logs: event.target.value === "logs",
  //     processes: event.target.value === "processes"
  //   });

  //   onChange({...value,
  //     queryText: event.target.value,
  //     metrics: event.target.value === "metrics",
  //     logs: event.target.value === "logs",
  //     processes: event.target.value === "processes"
  //   });

  //   onRunQuery();
  // }

  return (
    <>
      Select a graph:
      <select name="graph" value={query.queryText} onChange={onSelect}>&nbsp;
      { ['metrics', 'logs', 'processes'].map((option: string) =>
        <option key={option} value={option}>{option}</option>
      )}
      </select>
      {/* <Radio group={"report"} buttons={['metrics', 'logs', 'processes']} onClick={onRadioClick} /><br />
      Selection: {query.queryText} */}
    </>
  );
}

// function Radio(props: {group: string, buttons: string[], onClick: React.ChangeEventHandler<HTMLInputElement>}) {
//   const {group, buttons, onClick} = props;

//   const RadioInput = (props: { name: string; value: string; }) => {
//     const {name, value } = props;
//     const title = value[0].toUpperCase() + value.slice(1);
//     return (
//       <>
//         {title}: <input type="radio" name={name} value={value} onChange={onClick} />&nbsp;
//       </>
//     )
//   }

//   return (
//     <>
//       Select a graph type:<br/>
//       { buttons.map((button: string) =>
//         <RadioInput name={group} key={button} value={button} />
//       )}
//     </>
//   );
// }
