# Quilt Continuous Integration

This is a hacked together system for continuous integration testing for Quilt.

A lot of this code depends on the specific way Quilt logs things. For example,
if the output for the machine table were to change, we wouldn't be able to
get the IP addresses. We also watch the output for specific strings to trigger
our actions. The eventual official `dictl` tool will help in the future, though.

`quilt-tester` pulls the latest Quilt code from master, builds it, runs it with
minions on AWS, and then ssh's into each of the servers and runs a test script.
If the exit code of the test script is non-zero, then `quilt-tester` interprets
this test as having failed.

## Setup
[setup](vagrant/setup) should take care of everything.

For local instances:
```
cd vagrant
vagrant up && vagrant ssh
sudo ./setup # do this inside the VM
```

Make sure that if you're testing a fork of the project, that `quilt-tester` is included
as a user.

On AWS, just scp the setup file over and do the same thing.

The script will ask you a couple questions to configure itself.

The IP address is used to generate links to test results.

The slack channel is used to determine where to post results. You can
use `@$USER` (e.g. @kklin) to get the results DMd.

The "aws credentials cron" is a cron job that updates the aws credentials file
every night. Only use this if you're on AWS, and IAM roles are properly setup for the host.

The Vagrantfile tries to copy your aws credentials over automatically from the host computer,
so other than answering the initial questions, you shouldn't have to setup anything else.

## Usage
You can trigger a new test run by sending a GET or POST request to `http://$IP/cgi-bin/trigger_run`,
or manually SSHing in and running the `quilt-tester` script.

## Adding tests
To add a test, simply place it in the [tests](tests) directory. Exit with a
non-zero value if the test failed. Any output to stdout will be saved and
accessibly via the web.

## Logging
When things break, you can take a look at the `quilt-tester` log. Each run saves logs to
`/var/www/quilt-tester/$RUN/logs`.

A quick way to watch the logs is `tail -f "$(\ls -1dt /var/www/quilt-tester/*/ | head -n 1)/logs/run_out.log"`

## Security
`~/.ssh` must contain a private key associated with a GitHub account for the
Quilt repo. The associated public key must be in the spec file. A default key
and spec is provided in the repo.

Additionally, aws credentials must be present in `~/.aws`

## Apache
Apache needs to be configured to serve up `$WEB_ROOT` as defined in [tester](bin/tester).
The `setup` script should handle all the details.

## Modifying the testing interval
By default, the tests are automatically run every hour. You can tweak `quilt-tester`'s crontab
(using `crontab -e` as `quilt-tester`) to change the time interval.

## XXX:
- Setup post-commit hooks on Github for updating the tests folder and testing new merges
- Package quilt-controller, quilt-minion, and quilt-tester as containers for easy deployment
- Make sure we clean up after each test
    - Kill the minions using `aws cli` if the controller hangs
    - Bail early if things fail in the script
    - Assume failure if the minions don't connect after a certain timeout
- Ping kklin on slack if the tester seems to be failing (e.g. if we're timing out)
- Differentiate tests for master and workers?
- Create summary of names of tests that failed
- Do a security audit
- Allow only one instance of `quilt-tester` to run per machine at a time
- Remove the `quilt-tester` ssh key
