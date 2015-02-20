# YouTube Emacs Search

A small Go utility to search for the latest Emacs screencasts on
YouTube. This is a training project for me to learn Go.

## Setup OAuth credentials

To obtain the needed OAuth 2.0 token from YouTube you should read the
guide [Obtaining authorization credentials][1].

When creating a new client ID you have to choose the following:

- Application Type: **Installed application**
- Installed Application Type: **Other**

Copy the file `youtube-oauth-credentials.sample` to
`~/.config/youtube-oauth-credentials` and replace the sample data with
your OAuth credentials.

## Development

Before starting to develop on youtube-emacs-search you should make
sure to have `git-flow` installed and run the `boostrap` script. This
will setup `git-flow` with the default settings.

We recommend [`git-flow AVH Edition`][2]. For detailed installation
instructions have a look at the [git-flow Wiki][3].

### git-flow Crash Course

1. Start a new feature with `git-flow feature start FEATURE_NAME` (creates a new branch)
2. Hack on your feature
3. Finish your feature with `git-flow feature stop FEATURE_NAME` (merges the branch into `develop`)


[1]: https://developers.google.com/youtube/registering_an_application
[2]: https://github.com/petervanderdoes/gitflow/
[3]: https://github.com/petervanderdoes/gitflow/wiki
