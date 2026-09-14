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
			customCss: ['./src/styles/custom.css'],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/aj-abhinai/leap' }],
			sidebar: [
				{
					label: 'Guides',
					items: [
						// Each item here is one entry in the navigation menu.
						{ label: 'Introduction', slug: '' },
						{ label: 'Quickstart', slug: 'getting-started/quickstart' },
						{ label: 'Features', slug: 'features' },
						{ label: 'Development', slug: 'development' },
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
