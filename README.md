## Deployment

### Prerequisites

For deploying this project Ansible is required. Ansible docs can be founded here: https://docs.ansible.com/

Currently, we have an one playbook:

 - application.yml

`application.yml` preserves common environment and deploy selected version of application.

If you want to deploy application from a custom branch (default branch is 'master') -
go to playbooks/application.yml and change 'version' variable to what you need

### How to deploy a project to your's local machine into docker container

You can use docker image for testing a deploy project and ensure that all works correct.
Prepare related software, ansible and docker must be installed.

Just follow next steps:
 - update permissions for correct using rsa key by `sudo chmod 0600` for ./id_rsa & ./id_rsa.pub
 - create docker container by command `make build`
 - start running scripts by command `ansible-playbook --key-file id_rsa -i inventories/dev/hosts.yml playbooks/application.yml`
 - do `make ssh` to deep inside container and check that application is running

After deploy backEnd binary will be in /usr/local/bin folder, if you want to run it with some cli commands -
use `/usr/local/bin/{{ project_name }} {{  cli_command }}`

### How to deploy a project to a staging server

Prepare your's ssh config file, specify host, port and user name for server which will be used for deploy.

Example of specified ssh config:
```
cat ~/.ssh/config

Host demo.theflow.global
    HostName 123.123.123.123
    User root
    IdentityFile ~/.ssh/id_rsa

```

 - Write previously specified server name in the ./inventories/staging/hosts.yml
 - Start scripts by `ansible-playbook -i inventories/staging/host.yml playbooks/main.yml`
 - Go to your host in browser
 
After deploy backEnd binary will be in /usr/local/bin folder, if you want to run it with some cli commands -
use `/usr/local/bin/{{ project_name }} {{  cli_command }}`

### How to

#### Check your service status
  >systemctl status backoffice_app_api

#### Restart a service
  >systemctl restart backoffice_app_api

#### See service logs
  >journalctl --since 10:00 -u backoffice_app_api

examples of 'since' parameter:
 - yesterday
 - today
 - 2018-10-20
 - "2018-10-20 12:48"
 note that you and server most likely in different timezones
