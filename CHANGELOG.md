# Changelog

## [0.7.5](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.4...tfclassify-v0.7.5) (2026-03-30)


### Bug Fixes

* refresh Azure role data and action registry ([#152](https://github.com/jokarl/tfclassify/issues/152)) ([baad7f8](https://github.com/jokarl/tfclassify/commit/baad7f81b24b511262fdc7733c3fc8bd89ecfd35))

## [0.7.4](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.3...tfclassify-v0.7.4) (2026-03-27)


### Bug Fixes

* hide no-op resources in text output, show only active changes ([351bc74](https://github.com/jokarl/tfclassify/commit/351bc7462b44f8df6b8862fd79b35c675bb13cec))

## [0.7.3](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.2...tfclassify-v0.7.3) (2026-03-27)


### Bug Fixes

* show downgraded resources in verbose no-changes output ([acba5e8](https://github.com/jokarl/tfclassify/commit/acba5e8ef78f758cf0173051e39d28a716d4a6c4))

## [0.7.2](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.1...tfclassify-v0.7.2) (2026-03-26)


### Bug Fixes

* show "No resource changes" instead of classification description ([9ba052b](https://github.com/jokarl/tfclassify/commit/9ba052bf2b7a5639583d05f7f10a2a1f74ee0579))

## [0.7.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.7.0...tfclassify-v0.7.1) (2026-03-26)


### Bug Fixes

* treat all-no-op plans as no_changes after cosmetic filtering ([#144](https://github.com/jokarl/tfclassify/issues/144)) ([ef539c2](https://github.com/jokarl/tfclassify/commit/ef539c2412e7bb8b5678f9b9efe8771bf3d9efb8))

## [0.7.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.6.0...tfclassify-v0.7.0) (2026-03-25)


### Features

* add ignore_attributes to exclude cosmetic changes from classifi… ([#138](https://github.com/jokarl/tfclassify/issues/138)) ([85af62c](https://github.com/jokarl/tfclassify/commit/85af62c97a80f880bac749c87bdb809d5b804ad2))


### Bug Fixes

* refresh Azure role data and action registry ([#70](https://github.com/jokarl/tfclassify/issues/70)) ([d0b9390](https://github.com/jokarl/tfclassify/commit/d0b9390b465997e654de70c27b8029948464ac07))
* refresh Azure role data and action registry ([#98](https://github.com/jokarl/tfclassify/issues/98)) ([c3af999](https://github.com/jokarl/tfclassify/commit/c3af999d0b5694f8bf8113584869866995bcf306))
* use unique scope in combined-role-aggregation e2e to prevent parallel CI collisions ([53e9c09](https://github.com/jokarl/tfclassify/commit/53e9c0949e0642f651676df7e216a8940dc4ca7e))


### Dependencies

* **deps:** bump github.com/zclconf/go-cty from 1.17.0 to 1.18.0 ([#67](https://github.com/jokarl/tfclassify/issues/67)) ([38bde0a](https://github.com/jokarl/tfclassify/commit/38bde0a0e84ee815a98ebfa6fb9f5e4860b50279))
* **deps:** bump golang.org/x/sync from 0.19.0 to 0.20.0 ([#97](https://github.com/jokarl/tfclassify/issues/97)) ([6fabb0c](https://github.com/jokarl/tfclassify/commit/6fabb0cea89e394374d8348246b2c9c0ef5bc9dd))
* **deps:** bump google.golang.org/grpc from 1.79.1 to 1.79.3 ([#137](https://github.com/jokarl/tfclassify/issues/137)) ([aa24c7c](https://github.com/jokarl/tfclassify/commit/aa24c7c12599da13ade0d839c456d3ea0abf0d38))
* **deps:** bump google.golang.org/grpc from 1.79.1 to 1.79.3 in /sdk ([#136](https://github.com/jokarl/tfclassify/issues/136)) ([e90c7fc](https://github.com/jokarl/tfclassify/commit/e90c7fc91df295476f0e708cc2a1e9a475bd19ce))

## [0.6.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.5.2...tfclassify-v0.6.0) (2026-02-26)


### Features

* add combined exposure analysis ([#64](https://github.com/jokarl/tfclassify/issues/64)) ([569c32b](https://github.com/jokarl/tfclassify/commit/569c32b663facfb78c9c70df8ce710ec58b47657))
* add module-scoped rules, drift classification, and topology analysis ([#65](https://github.com/jokarl/tfclassify/issues/65)) ([87b33bd](https://github.com/jokarl/tfclassify/commit/87b33bd3a78750abdec93d3a1758b897f6a88dee))


### Bug Fixes

* remove shallow analyzers, fix code defects, harden testing ([#62](https://github.com/jokarl/tfclassify/issues/62)) ([74b15c4](https://github.com/jokarl/tfclassify/commit/74b15c4e0dfa45288c7c6b8c062f8c441918b364))

## [0.5.2](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.5.1...tfclassify-v0.5.2) (2026-02-23)


### Bug Fixes

* refresh Azure role data and action registry ([#51](https://github.com/jokarl/tfclassify/issues/51)) ([a132079](https://github.com/jokarl/tfclassify/commit/a132079321681dcb00d7fed24d7fe6e32c10f952))


### Dependencies

* **gomod:** bump github.com/hashicorp/go-version from 1.7.0 to 1.8.0 ([#46](https://github.com/jokarl/tfclassify/issues/46)) ([cf733d8](https://github.com/jokarl/tfclassify/commit/cf733d8befa100cc505c6b1cd10c87c77f0f7049))
* **gomod:** bump github.com/zclconf/go-cty from 1.16.4 to 1.17.0 ([#47](https://github.com/jokarl/tfclassify/issues/47)) ([477cf04](https://github.com/jokarl/tfclassify/commit/477cf046f86391a9d9319de81ecbaf5106fdcc2b))

## [0.5.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.5.0...tfclassify-v0.5.1) (2026-02-22)


### Bug Fixes

* address v0.5.0 critique findings ([#42](https://github.com/jokarl/tfclassify/issues/42)) ([74abb72](https://github.com/jokarl/tfclassify/commit/74abb72dc035d0dda0c3fc93b1970b154330f974))

## [0.5.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.4.0...tfclassify-v0.5.0) (2026-02-22)


### Features

* SARIF 2.1.0 output format (CR-0033) ([#39](https://github.com/jokarl/tfclassify/issues/39)) ([f23854d](https://github.com/jokarl/tfclassify/commit/f23854dae8be20bf5eae562b8c14aba29f13c9cc))

## [0.4.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.3.0...tfclassify-v0.4.0) (2026-02-20)


### Features

* blast radius analyzer, not_actions, evidence artifacts (CR-0029/0030/0032) ([#37](https://github.com/jokarl/tfclassify/issues/37)) ([1db2990](https://github.com/jokarl/tfclassify/commit/1db29906fc7a2bec2af2d658c69cd80bfd2d735f))

## [0.3.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.2.1...tfclassify-v0.3.0) (2026-02-18)


### Features

* add validate and explain subcommands (CR-0025, CR-0026) ([#35](https://github.com/jokarl/tfclassify/issues/35)) ([012ea82](https://github.com/jokarl/tfclassify/commit/012ea820109756a1cfb78fcfbca0cfbd22fc90fa))

## [0.2.1](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.2.0...tfclassify-v0.2.1) (2026-02-18)


### Performance Improvements

* optimize hot paths and reduce allocations across codebase ([#31](https://github.com/jokarl/tfclassify/issues/31)) ([8ffe569](https://github.com/jokarl/tfclassify/commit/8ffe56999bde1523d2c623946c01a7a902181842))

## [0.2.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.1.0...tfclassify-v0.2.0) (2026-02-17)


### Features

* classification-scoped plugin config and --detailed-exitcode flag ([#14](https://github.com/jokarl/tfclassify/issues/14)) ([128646b](https://github.com/jokarl/tfclassify/commit/128646b8d33bb64cf585bc3d9460bee98a96d924))
* pattern-based control-plane and data-plane detection (CR-0027, CR-0028) ([#22](https://github.com/jokarl/tfclassify/issues/22)) ([3b4ef50](https://github.com/jokarl/tfclassify/commit/3b4ef50f37ad4aade4dadb03abb2e655a3de52fe))

## [0.1.0](https://github.com/jokarl/tfclassify/compare/tfclassify-v0.0.1...tfclassify-v0.1.0) (2026-02-14)


### Features

* implement tfclassify MVP with plugin architecture, Azure deep inspection, and CI/CD ([#2](https://github.com/jokarl/tfclassify/issues/2)) ([8d15cd6](https://github.com/jokarl/tfclassify/commit/8d15cd65748b2465b96ff1af93fce90598dcf84b))


### Bug Fixes

* bump Go to 1.26.0 to resolve 16 stdlib vulnerabilities ([d02e40e](https://github.com/jokarl/tfclassify/commit/d02e40e690ce0cd000054108852568251809c377))
* honor TERRAFORM_PATH override when path does not exist ([6d9d09a](https://github.com/jokarl/tfclassify/commit/6d9d09a57ed2f9cddcc6c76d5b5f3c9b3edd7aae))
