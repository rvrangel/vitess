/**
 * Copyright 2021 The Vitess Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import cx from 'classnames';

import style from './Pip.module.scss';

interface Props {
    className?: string;
    state?: PipState;
}

export type PipState = 'primary' | 'success' | 'warning' | 'danger' | 'throttled' | null | undefined;

export const Pip = ({ className, state }: Props) => {
    return <div className={cx(className, style.pip, state && style[state])} />;
};
