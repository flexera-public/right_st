# Releasing `right_st`

This is how to release a new version of `right_st`:

1. Verify that all of the tests pass:
  * Linux and Mac OS X [![Travis CI Build Status](https://travis-ci.org/rightscale/right_st.svg?branch=master)](https://travis-ci.org/rightscale/right_st?branch=master)
  * Windows [![AppVeyor Build Status](https://ci.appveyor.com/api/projects/status/github/rightscale/right_st?branch=master&svg=true)](https://ci.appveyor.com/project/RightScale/right-st?branch=master)
2. Make sure the [ChangeLog](https://github.com/rightscale/right_st/blob/master/ChangeLog.md) and
  [README](https://github.com/rightscale/right_st/blob/master/README.md) are up to date.
3. Create a tag of the form `vX.Y.Z` where `X`, `Y`, and `Z` are the major, minor, and patch versions respectively:

  ```bash
  git checkout master
  git pull
  git tag --annotate --message='Release version vX.Y.Z' vX.Y.Z
  git push --tags
  ```
4. Create a [GitHub release](https://github.com/rightscale/right_st/releases) from the tag with the ChangeLog contents
  as the description.

## Testing the release

TBD: should include testing binaries for Linux, Mac OS X, and Windows
