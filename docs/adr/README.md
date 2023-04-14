---
order: 1
parent:
order: false
---

# Architecture Decision Records (ADR)

This is a location to record all high-level architecture decisions in this repository.

You can read more about the ADR concept in this [blog post](https://product.reverb.com/documenting-architecture-decisions-the-reverb-way-a3563bb24bd0#.78xhdix6t).

An ADR should provide:

- Context on the relevant goals and the current state
- Proposed changes to achieve the goals
- Summary of pros and cons
- References
- Changelog

Note the distinction between an ADR and a spec. The ADR provides the context, intuition, reasoning, and
justification for a change in architecture, or for the architecture of something
new. The spec is much more compressed and streamlined summary of everything as
it stands today.

If recorded decisions turned out to be lacking, convene a discussion, record the new decisions here, and then modify the code to match.

Note the context/background should be written in the present tense.

To start a new ADR, you can generate a new file with the following command:

```bash
make adr-gen NUM=001 TITLE=my-adr-title
```

## Table of Contents
