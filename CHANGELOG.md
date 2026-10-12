# Changelog

## [1.0.0](https://github.com/dever-labs/resctl/compare/v0.5.0...v1.0.0) (2026-10-12)


### Features

* add confirmation prompt to resctl uninstall ([#32](https://github.com/dever-labs/resctl/issues/32)) ([3070d1e](https://github.com/dever-labs/resctl/commit/3070d1ed1955c6545f6d8e46f453a46a58aa6da8))
* add resctl completion command for bash/zsh/fish ([#33](https://github.com/dever-labs/resctl/issues/33)) ([d119931](https://github.com/dever-labs/resctl/commit/d1199318b2f797332eef90317fd85d22ea920d4a))
* add resctl displays command and --display validation ([#28](https://github.com/dever-labs/resctl/issues/28)) ([7b19be3](https://github.com/dever-labs/resctl/commit/7b19be372e0b00471fbb839d7ba3fa61e3f5fd88))
* add version field to toggle state file schema ([#30](https://github.com/dever-labs/resctl/issues/30)) ([55e270f](https://github.com/dever-labs/resctl/commit/55e270ff700b4e59a51bdbd499dffc29184402cd))


### Bug Fixes

* **ci:** prevent script injection via github.ref_name in release.yml ([#24](https://github.com/dever-labs/resctl/issues/24)) ([5c4d54a](https://github.com/dever-labs/resctl/commit/5c4d54a453c8f6f8785acfefe6daef1533522e70))
* make state file writes atomic ([#29](https://github.com/dever-labs/resctl/issues/29)) ([787a25a](https://github.com/dever-labs/resctl/commit/787a25a119b37f04607b208c171db6498e526486))
* remap toggle CurrentIndex by entry, not position, after filtering ([#34](https://github.com/dever-labs/resctl/issues/34)) ([5dddd33](https://github.com/dever-labs/resctl/commit/5dddd3348b9f04ed71144ec32384b1d2fdc07107))
* suggest xrandr/XWayland fallback for unknown Wayland compositors ([#26](https://github.com/dever-labs/resctl/issues/26)) ([6899a01](https://github.com/dever-labs/resctl/commit/6899a01c6e2448467b763ce3c6649a93927126ff))
* validate saved toggle resolutions still exist before use ([#31](https://github.com/dever-labs/resctl/issues/31)) ([da8208c](https://github.com/dever-labs/resctl/commit/da8208c8937f171d2829091e4ad65035d2a9b19d))

## [0.5.0](https://github.com/dever-labs/resctl/compare/v0.4.0...v0.5.0) (2026-10-11)


### Features

* add --list and --clear to 'resctl toggle' ([11fa37e](https://github.com/dever-labs/resctl/commit/11fa37e18be6239b696218ff6c4ac3f29e1efc9b))
* add --list and --clear to 'resctl toggle' ([7ac965d](https://github.com/dever-labs/resctl/commit/7ac965d17789101f28fe4eaec5a8bac45b26a6a8)), closes [#2](https://github.com/dever-labs/resctl/issues/2)
* detect no-op in 'resctl set' and add --force ([535d4b6](https://github.com/dever-labs/resctl/commit/535d4b6a5c1265e736c684bda5bf1b25f91449e0))
* detect no-op in 'resctl set' and add --force ([827858e](https://github.com/dever-labs/resctl/commit/827858e09b06313520967952b1bdd18c92f01c09)), closes [#8](https://github.com/dever-labs/resctl/issues/8)
* group refresh rates under resolutions in 'resctl list' ([ea4560e](https://github.com/dever-labs/resctl/commit/ea4560ef4ae8fdfd1b962ff8d474d4567f69283b))
* group refresh rates under resolutions in 'resctl list' ([a2e1300](https://github.com/dever-labs/resctl/commit/a2e130024e81bd591d551103152df46f252fbe9f)), closes [#5](https://github.com/dever-labs/resctl/issues/5)


### Bug Fixes

* configure dependabot for gomod and github-actions ([95bf3e8](https://github.com/dever-labs/resctl/commit/95bf3e896445e5231f1c869b6fc2497f6fa9ae9f))
* configure dependabot for gomod and github-actions ([492e28a](https://github.com/dever-labs/resctl/commit/492e28a59133bbff6b6c15b8a5d8a5388df88ed1))
* tighten toggle --list/--clear edge cases found in review ([bf2517d](https://github.com/dever-labs/resctl/commit/bf2517db138a4586961a151d7e05815e80550e5b))
