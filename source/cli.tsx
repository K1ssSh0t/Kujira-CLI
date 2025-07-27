#!/usr/bin/env node
import React from 'react';
import { render } from 'ink';
import meow from 'meow';
import App from './app.js';

const cli = meow(
	`
  Usage
    $ Kujira-CLI

  Options
    --provider  Anime provider (animeflv or tioanime)

  Examples
    $ Kujira-CLI --provider=tioanime
`,
	{
		importMeta: import.meta,
		flags: {
			provider: {
				type: 'string',
			},
		},
	}
);

render(<App provider={cli.flags.provider} />);