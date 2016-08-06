unreleased changes
-------------------
* Add `--script-path` argument to specify where to download RightScripts relative
  to the ServerTemplate download location ([#28])
* Fix a panic in ServerTemplate validate when the RightScript name is not known
  yet
* If multiple RightScripts in a ServerTemplate contain an attachment with the
  same name, now download those attachments to different directories ([#19])

[#28]: https://github.com/rightscale/right_st/pull/28
[#19]: https://github.com/rightscale/right_st/pull/19

v1.3.0 / 2016-07-15
-------------------
* Support for "published" or "external" MCIs. right_st will attempt to check for
  and use MultiCloudImages from the MultiCloud Marketplace if they exist ([#10])
* Support for full definition of MCIs in the ServerTemplate YAML file, including
  managing MCI settings and tags ([#11])
* Support specifying Packages field in RightScript comment metadata

[#10]: https://github.com/rightscale/right_st/issues/10
[#11]: https://github.com/rightscale/right_st/issues/11

v1.2.0 / 2016-06-05
-------------------
* Add ability to use "published" aka "external" RightScripts. These are
  RightScripts published in the MultiCloud Marketplace. These are linked to
  instead of downloaded to disk ([#16])

[#16]: https://github.com/rightscale/right_st/issues/16

v1.1.0 / 2016-04-15
-------------------
* Various fixes: Can specify a directory to download to. Better error messages.
* Now display script body in RightScript show command.
* Added --force option to scaffold to rescaffold an existing script [#5]
* Will clean up RightScript metadata on download if it is incorrect
* Attachments with the same name but belonging to different Rightscript will
  throw an error ([#9])
* Attachment names can now have absolute paths, or relative path components
  added in
* Fix [#13] with some trickery. Add a temporary MCI before deleting then adding
  the new MCI revision.
* Fix [#17] fully by only allowing actual host names.

[#5]: https://github.com/rightscale/right_st/issues/5
[#9]: https://github.com/rightscale/right_st/issues/9
[#13]: https://github.com/rightscale/right_st/issues/13
[#17]: https://github.com/rightscale/right_st/issues/17

v1.0.2 / 2016-04-07
-------------------
* Build with Go 1.6 on Travis CI.
* Fix [#14] by only doing an Inputs MultiUpdate when there are actually inputs
  to update.
* Actually unset inputs that are removed by getting the old inputs from a
  ServerTemplate before and setting any that are not being updated to "inherit".

[#14]: https://github.com/rightscale/right_st/issues/14

v1.0.1 / 2016-03-09
-------------------
* Fix a cosmetic bug where attachment downloads on Windows would display as
  `Downloading attachment into attachments/attachments\filename.txt` instead of
  `Downloading attachment into attachments/filename.txt`

v1.0.0 / 2016-03-03
-------------------
* Initial release
