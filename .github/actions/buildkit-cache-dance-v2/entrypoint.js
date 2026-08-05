// Forked from https://github.com/pyTooling/Actions/blob/v0.4.6/with-post-step/main.js
// The following copyright header is from the upstream.

/* ================================================================================================================== *
 * Authors:                                                                                                           *
 *   Unai Martinez-Corral                                                                                             *
 *                                                                                                                    *
 * ================================================================================================================== *
 * Copyright 2021-2022 Unai Martinez-Corral <unai.martinezcorral@ehu.eus>                                             *
 * Copyright 2022 Unai Martinez-Corral <umartinezcorral@antmicro.com>                                                 *
 *                                                                                                                    *
 * Licensed under the Apache License, Version 2.0 (the "License");                                                    *
 * you may not use this file except in compliance with the License.                                                   *
 * You may obtain a copy of the License at                                                                            *
 *                                                                                                                    *
 *     http://www.apache.org/licenses/LICENSE-2.0                                                                     *
 *                                                                                                                    *
 * Unless required by applicable law or agreed to in writing, software                                                *
 * distributed under the License is distributed on an "AS IS" BASIS,                                                  *
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.                                           *
 * See the License for the specific language governing permissions and                                                *
 * limitations under the License.                                                                                     *
 *                                                                                                                    *
 * SPDX-License-Identifier: Apache-2.0                                                                                *
 * ================================================================================================================== *
 *                                                                                                                    *
 * Context:                                                                                                           *
 * * https://github.com/docker/login-action/issues/72                                                                 *
 * * https://github.com/actions/runner/issues/1478                                                                    *
 * ================================================================================================================== */
const { spawn } = require("child_process");
const { appendFileSync } = require("fs");
const { EOL } = require("os");

function run(cmd) {
  const subprocess = spawn(cmd, { stdio: "inherit", shell: false });
  subprocess.on("exit", (exitCode) => {
    process.exitCode = exitCode;
  });
}

const key = "POST"

if ( process.env[`STATE_${key}`] !== undefined ) { // Are we in the 'post' step?
  run(__dirname+"/post");
} else { // Otherwise, this is the main step
  appendFileSync(process.env.GITHUB_STATE, `${key}=true${EOL}`);
  run(__dirname+"/main");
}
