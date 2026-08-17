// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	integrations: [
		starlight({
			title: 'Leap',
			logo: {
				src: './src/assets/logo.png',
			},
			favicon: '/favicon.png',
			sidebar: [
				{
					label: 'Guides',
					items: [
						// Each item here is one entry in the navigation menu.
						{ label: 'Introduction', slug: '' },
						{ label: 'Quickstart', slug: 'getting-started/quickstart' },
					],
				},
				{
					label: 'Reference',
					items: [{ label: 'API Overview', slug: 'api-reference/overview' }],
				},
			],
		}),
	],
});
