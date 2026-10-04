# To Do List

## Planned

- Make the app fill the screen (not fullscreen mode, but as big as it can be to show as much content as possible/needed)

## Potential

- Add an option to detect and handle covers and credits images
  - Make this a setting that can be toggled in the View menu and stored globally. It will be application level behaviour, not archive level behaviour. Do not create a toolbar icon for it
  - Detect cover files by name
    - Sometimes cover files contain the word 'cover'
    - If a file contains the text 'cover' and there is no metadata indicating which file is the cover, then treat that file as the cover and put it first. If there happen to be multiple files fitting this critierai, put them all at the front
  - Detect credits images
    - Sometimes an archive has an file with a name containing the text 'credits'. If so, treat them as credit content and put sort them to the end of the files
- Image smoothing/enhancement
  - This is quite speculative, but I'm just wondering if it might be possible to do something simple to image the apperance of older manga scans that may be a bit pixelated or low resolution

## One Day

- Build and test a Linux version