# Rancher Vagrant

***This is a fork and modification of https://github.com/coreos/coreos-vagrant, credit goes to CoreOS for the real work***

## Single node setup

Just run `vagrant up` and the UI/API will be available at port 8080.

## Multi node setup

Copy `vagrant/config.rb.sample` to `vagrant/config.rb` and then change `num_instances`.
Then go to https://discovery.etcd.io/new and get a new token to put in `vagrant/user-data`.
