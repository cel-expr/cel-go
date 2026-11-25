# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Build rules for generating CEL environment conformance test targets."""

load("@rules_shell//shell:sh_test.bzl", "sh_test")
load("//conformance:conformance_test.bzl", "expand_tests_to_skip")

def cel_env_conformance_test(name, test_files, skip_tests = [], **kwargs):
    """Generates an env conformance test target.

    Args:
        name: Name of the test target.
        test_files: List of targets or files representing the test suite textprotos.
        skip_tests: List of test cases (or suite/section prefixes) to skip.
        **kwargs: Additional arguments passed to the underlying sh_test.
    """
    if not test_files:
        fail("test_files must be non-empty")

    go_default_test_label = Label(":go_default_test")
    paths = ["$(rlocationpath {})".format(f) for f in test_files]
    args = [
        "$(location {})".format(go_default_test_label),
        "--tests={}".format(",".join(paths)),
    ]

    if skip_tests:
        args.append("--skip_tests={}".format(",".join(expand_tests_to_skip(skip_tests))))

    sh_test(
        name = name,
        size = kwargs.pop("size", "small"),
        srcs = [Label("//conformance:conformance_test.sh")],
        args = args,
        data = [go_default_test_label] + test_files,
        **kwargs
    )
