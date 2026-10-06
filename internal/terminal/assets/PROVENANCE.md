# zsh-syntax-highlighting

Embedded upstream runtime files only (tests/docs omitted), BSD 3-Clause license
retained as `zsh-syntax-highlighting/COPYING.md`.

- Source: https://github.com/zsh-users/zsh-syntax-highlighting
- Release: 0.8.0
- Immutable commit: `db085e4661f6aafd24e5acb5b2e17e4dd5dddf3e`
- Upstream commit ZIP SHA256:
  `661296962f10f35a303f41d8ee61ec50d3750006fe875903bb86bf005c558f77`

Only trailing whitespace/newline normalization differs from upstream. Runtime
semantics are unchanged. Session colors are configured in an
owned temporary profile, after sourcing the user's existing zsh startup files.
