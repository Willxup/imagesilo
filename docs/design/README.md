# ImageSilo design study

This directory is an isolated, interactive design study. It is not the application and does not make API calls. File names, counts, upload progress and copy notifications are sample data.

Open `index.html` through a local static server rooted at the repository. The review bar switches three compositions: Studio (labeled side navigation), Gallery (top navigation), and Darkroom (dark photographic canvas). Inspect upload, library, selection, image detail, filters, mappings, tokens, system and settings before choosing the implementation direction. `auth.html` studies login and initial setup. `decisions.md` records the design rationale, functional mapping and implementation sequence.

The implementation combines the Studio composition with the Darkroom palette for dark mode. `acceptance.md` distinguishes prototype observations from checks against the running application. The production source is under `web/src`; this study remains separate from its assets and API flows.

Sample photographs, solely for evaluating image-heavy compositions:

| Local asset | Photographer | Original |
| --- | --- | --- |
| sample-12.jpg | Paul Jarvis | https://unsplash.com/photos/I_9ILwtsl_k |
| sample-14.jpg | Paul Jarvis | https://unsplash.com/photos/IQ1kOQTJrOQ |
| sample-15.jpg | Paul Jarvis | https://unsplash.com/photos/NYDo21ssGao |
| sample-17.jpg | Paul Jarvis | https://unsplash.com/photos/Ven2CV8IJ5A |
| sample-18.jpg | Paul Jarvis | https://unsplash.com/photos/Ps2n0rShqaM |
| sample-20.jpg | Aleks Dorohovich | https://unsplash.com/photos/nJdwUHmaY8A |

Sized copies obtained via Lorem Picsum. They are not added to the production application or a user's library. The ImageSilo logo remains the repository's existing asset.
