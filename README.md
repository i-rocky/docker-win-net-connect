# docker-win-net-connect

The client is already available in docker hub but if you want you can build it yourself. 

First build the client, use the name `app`, see Makefile. Then build the container using the given Dockerfile.


Build the main app for windows. The binaries provided in `bin` directory are x64 only. 

You can grab any version from wireguards official windos builds as your wish and build this app for your preferred architecture.

Commands:
* Installing `<file>.exe install`
* Uinstalling `<file>.exe uninstall` or `<file>.exe remove`
* Starting service `<file>.exe start`
* Stopping service `<file>.exe stop`

  > Must stop the service before uninstalling


Use without installing `<file>.exe debug`
