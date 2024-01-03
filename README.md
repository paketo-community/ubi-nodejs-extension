# Paketo Node.js Extension for ubi

## `docker.io/paketocommunity/ubi-nodejs-extension`

The Node.js Extension for [UBI](https://www.redhat.com/en/blog/introducing-red-hat-universal-base-image) allows builders to be created which build Node.js applications on top of Red Hat's Node.js ubi containers. For example [ubi8/nodejs-20-minimal](https://catalog.redhat.com/software/containers/rhel8/nodejs-20-minimal/6476fa2bb83e400ee9ce2332).

## Integration

The ubi Node.js extension provides node and npm as dependencies. Downstream buildpacks, like Yarn Install CNB or NPM CNB, can require the node dependency by generating a Build Plan TOML file that requires node and or npm.

The extension integrates with the existing Paketo buildpacks so that building your application will have the same experience as building with non ubi stacks. The main difference is that node.js and npm will be provided by the extension instead of the node-engine build pack.

## Usage

### Install Dependencies

- [pack cli](https://buildpacks.io/docs/tools/pack/)
- [docker](https://docs.docker.com/engine/install/)

### Configure

- Enable [experimental](https://buildpacks.io/docs/tools/pack/cli/pack_config_experimental/) features of `pack cli` by running `pack config experimental true`. This is needed because extensions are currently experimental.

### Build and Run the app

Build your app where `test-app` will be the name of the image built and `app-dir` is the directory that contains your Node.js application.

```sh
pack build test-app-name --path ./app-dir --builder paketocommunity/builder-ubi-base
```

Run your application with `docker run -p 8080:8080 test-app` replacing `8080:8080` with the port on which your application listens.

Access your running application and enjoy :)

### Add more buildpacks on the build process

Nothing stops you from adding [more buildpacks](https://github.com/orgs/paketo-buildpacks/repositories) during the build process such as [procfile](https://github.com/paketo-buildpacks/procfile), [ca-certificates](https://github.com/paketo-buildpacks/ca-certificates) etc.

### Build without the full builder

The [build-ubi-base](https://github.com/paketo-community/builder-ubi-base) builder [has a list](https://github.com/paketo-community/builder-ubi-base/blob/main/builder.toml) of buildpacks and an extension participating during build. You can also use the [buildpackless ubi builder](https://github.com/paketo-community/builder-ubi-buildpackless-base) which has no buildpacks or the extension participating during build and include only the ones you want as demonstrated on below example for a Node.js app.

```sh
pack build test-app \
   --path ./app-dir \
   --extension docker.io/paketocommunity/ubi-nodejs-extension \
   --buildpack paketo-buildpacks/node-engine \
   --buildpack paketo-buildpacks/npm-install \
   --buildpack paketo-community/build-plan \
   --builder paketocommunity/builder-ubi-buildpackless-base
```

### Install a Specific a Node Engine Version

UBI only supports the latest minor version of each Node.js stream currently available in the UBI version.

At the time of writing, ubi8 supports the Node.js 16, 18, and 20 streams. For example, if the latest Node.js version for the 16 stream in ubi8 is 16.10.1 then that is your **only option** when requesting the Node.js 16.x stream. Therefore we suggest that you request the Node.js version such that it will accept any version of the stream you want to use with something like `~16`.

The extension prioritizes the versions specified in each possible configuration location with the following precedence, from highest to lowest:

- Set the `$BP_NODE_VERSION` environment variable at build time

```bash
pack build test-app-name \
   --path ./app-dir \
   --builder paketocommunity/builder-ubi-base \
   --env BP_NODE_VERSION="~20"
```

- Set the `$BP_NODE_VERSION` through a [`project.toml` file](https://github.com/buildpacks/spec/blob/main/extensions/project-descriptor.md#iobuildpacksbuildenv-optional). 

```toml
[ _ ]
schema-version = "0.2"

[[io.buildpacks.build.env]]

name = 'BP_NODE_VERSION'
value = '~20'

```

- Set the node version in `package.json`:

  ```json
   "engines": {
      "node": "~16"
   },
  ```

- Set the node version via an `.nvmrc` file located at the application root directory

- Set the node version via an `.node-version` file located at the application root directory

### Specifying a project path

To specify a project subdirectory to be used as the root of the app, please use the `BP_NODE_PROJECT_PATH` environment variable at build time either directly (ex. `pack build my-app --env BP_NODE_PROJECT_PATH=./src/my-app`) or through a [project.toml file](https://github.com/buildpacks/spec/blob/main/extensions/project-descriptor.md). This could be useful if your app is a part of a monorepo.

## Run Tests

To run all unit tests, run:

```sh
./scripts/unit.sh
```

To run all integration tests, run:

```sh
./scripts/integration.sh
```

## Package buildpack (Generate .tgx & .cnb files)

To generate `buildpackage.cnb` and `buildpack.tgz` files

```
./scripts/package.sh --version 0.0.1
```
