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
