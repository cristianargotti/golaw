# The fixture profile

This profile only adds checks to the core of golaw; it removes none and relaxes none. The remote gate of a repository enrolled in it is whatever the adopting platform runs: without a remote stage that runs the pinned kit, the checks of the kit run only on the machines whose developers armed the git rungs with golaw setup, and the pipeline of the platform is the only gate. A pull request is ready when that pipeline's run of make ci is green.
