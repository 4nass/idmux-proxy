---
title: Edit this site
---

# Edit this site

The site is made from Markdown files in the `docs/` folder. GitHub builds the
site after a change reaches `main`.

## Edit on the web

1. Open the [IdMux repository](https://github.com/4nass/idmux-proxy).
2. Open the `docs/` folder.
3. Open a Markdown page and select the pencil button.
4. Make a small change.
5. Open a pull request.

This is the easiest way to fix a sentence or add a short note. You need write
access to edit the repository directly. Other contributors can use a fork and
a pull request.

## Edit with `gh`

Install and sign in to the free [GitHub CLI](https://cli.github.com/), then run:

```text
gh auth login
gh repo clone 4nass/idmux-proxy
cd idmux-proxy
git switch -c docs/my-change
```

Edit a file in `docs/`, then run:

```text
git add docs/
git commit -m "docs: explain my change"
git push --set-upstream origin docs/my-change
gh pr create --fill
```

The pull request runs the normal checks. After it is merged, GitHub Pages
publishes the new page.

## Keep the pages simple

- Use short sentences.
- Use one idea per paragraph.
- Add a command when a reader must run something.
- Do not add secrets or private data.
- Keep links working.

The website URL is:

<https://4nass.github.io/idmux-proxy/>
