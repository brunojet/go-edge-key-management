# Changelog

## [1.2.1](https://github.com/brunojet/go-edge-key-management/compare/v1.2.0...v1.2.1) (2026-06-06)


### Documentation

* comprehensive documentation overhaul ([9f3e112](https://github.com/brunojet/go-edge-key-management/commit/9f3e11237a9e00cce500f94d8c9156b3d7d30c0d))
* comprehensive documentation overhaul ([ca2066c](https://github.com/brunojet/go-edge-key-management/commit/ca2066c1a6d45eb25250f1194d0b253c5e0ea42b))
* improve README with clear navigation and getting started ([00da176](https://github.com/brunojet/go-edge-key-management/commit/00da17638108fc8ad231631fdadc20b5cd41a044))
* update risk analysis with completion status and doc links ([0215590](https://github.com/brunojet/go-edge-key-management/commit/02155905803b56d8191d6820bda6a69830730349))

## [1.2.0](https://github.com/brunojet/go-edge-key-management/compare/v1.1.0...v1.2.0) (2026-06-06)


### Features

* Improve test coverage to 97.1% (&gt;=96% requirement) ([0f78de7](https://github.com/brunojet/go-edge-key-management/commit/0f78de730ef649e8767fee9d71f070e30dc70239))


### Bug Fixes

* remove coverage file before mkdir in coverage script ([77de25d](https://github.com/brunojet/go-edge-key-management/commit/77de25d1cc209c7710f8613b89b2309729445ca0))


### Documentation

* Add upgrade guide for go-infra-adapters v4.0.0 ([74a64e7](https://github.com/brunojet/go-edge-key-management/commit/74a64e74ae2c7f40bf8a6a552e3763f66ef5891d))


### Code Refactoring

* code formatting, terraform hardening, arm64 support ([fff276b](https://github.com/brunojet/go-edge-key-management/commit/fff276bb7e3af2ae130812536a5d31152345d8ef))
* terraform improvements - arm64, logging, sanitization ([14000ea](https://github.com/brunojet/go-edge-key-management/commit/14000ea6a50bf7f1430fd337d5d9eaee4690cb75))

## [1.1.0](https://github.com/brunojet/go-edge-key-management/compare/v1.0.0...v1.1.0) (2026-06-04)


### Features

* Move public key creation to createSecret with sanitization ([eaa48be](https://github.com/brunojet/go-edge-key-management/commit/eaa48bee3a77c617d575decdd33b3122049efd55))
* Move public key creation to createSecret with sanitization on errors ([b8a80a5](https://github.com/brunojet/go-edge-key-management/commit/b8a80a5c357f09e84da38711e8e25f91ea9b799d))
* Simplify stale AWSPENDING cleanup with unconditional discard ([fa91acd](https://github.com/brunojet/go-edge-key-management/commit/fa91acd4c3a985d98972e55ac7d1be98ca035830))
* upgrade go-infra-adapters to v3.1.0+ with 100% test coverage ([b5f1b13](https://github.com/brunojet/go-edge-key-management/commit/b5f1b132b798e84eab55de7deeca0a312cedd2bc))
* upgrade to go-infra-adapters v3.0.0 with public HealthCheck API ([b2f48b4](https://github.com/brunojet/go-edge-key-management/commit/b2f48b46cc05d9134d4c6e8a017761d23ea382ad))


### Bug Fixes

* correct coverage requirement to 96% (actual: 96.5%) ([a4d18e0](https://github.com/brunojet/go-edge-key-management/commit/a4d18e02d6cbbeb5a68ba28bc6b1be624630e360))
* linting issues - errcheck, goimports, misspell, revive, staticcheck ([3fd7269](https://github.com/brunojet/go-edge-key-management/commit/3fd7269463637c9d3864ba1daaf337cd4a29704f))
* lower coverage requirement from 100% to 97% ([d303488](https://github.com/brunojet/go-edge-key-management/commit/d303488ad3eec1f5132417bb44b888086a7c2d46))
* remove invalid TestCreateSecret_GetVersionError test ([a556217](https://github.com/brunojet/go-edge-key-management/commit/a556217ad2e762ecc07123ac33b19e7441b8acc3))
* resolve all linting issues - goimports and gosec ([8525cd6](https://github.com/brunojet/go-edge-key-management/commit/8525cd683437c7538d2aca9d8cb407092e531656))
* validate complete key pair in pending version to handle stale AWSPENDING ([79035b6](https://github.com/brunojet/go-edge-key-management/commit/79035b6ce8d54b3a83f1f703aca73fe3a8e585b8))


### Code Refactoring

* extract discardPublicKey helper and fix error handling ([322baf7](https://github.com/brunojet/go-edge-key-management/commit/322baf70ec7afea5acc583b042079001e694af5f))
* migrate to go-infra-adapters v2.0.0 with adapter pattern ([c9c0a9c](https://github.com/brunojet/go-edge-key-management/commit/c9c0a9c87cd6175e8b1bb426aeb63de17d3dc849))
* use CDN and secret services from go-infra-adapters adapter ([e78e1a9](https://github.com/brunojet/go-edge-key-management/commit/e78e1a9caeb3992a9f0676925adc769663230896))
* validate all required fields in SecretPayload.IsValid() ([51fe180](https://github.com/brunojet/go-edge-key-management/commit/51fe1802508ae5ec93d4bfc97d2b74ca8363c270))
