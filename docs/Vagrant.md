# Vagrant

The Vagrant backend allows Quilt to operate local virtual machines in a Quilt
cluster, just as if they were running in a cloud provider.  This can be quite
useful for development, debugging, or experimentation with new systems.  The
compute _and network_ are deployed locally, _exactly_ as they would be in a
production environment.

## Installation

Vagrant is a frontend wrapper for Desktop/Laptop hypervisors, with support for
quite a different [providers](https://www.vagrantup.com/docs/providers/).
Currently, Quilt only supports Virtual Box, though support for other providers
should be to implement. Feel free to file an issue or [contact
us](README.md#contact) if other providers are important to you.



If you haven't already, you will need to install Virtual Box by
[downloading](https://www.virtualbox.org/wiki/Downloads) the installer and
running it.  That done, next you will
[install Vagrant](https://www.vagrantup.com/docs/installation/).  On most
platforms, this is a simple matter of
[downloading](https://www.vagrantup.com/downloads.html) the vagrant installer
and executing it.

With Virtual Box and Vagrant installed, you're ready to run your first Vagrant
Quilt Infrastructure!

## Example Specification

To get started with Vagrant, follow the instructions found in
[GettingStarted.md](GettingStarted.md).  These instructions will work as
written, requiring only one minor tweak.  In order to instruct Quilt to deploy
on Vagrant instead of Amazon EC2, tweak
[specs/example.spec](../specs/example.spec) to specify the
Vagrant provider.  Specifically change the line `(define Provider "Amazon")` to
be, instead, `(define Provider "Vagrant")`.
