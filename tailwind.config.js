/** @type {import('tailwindcss').Config} */
const colors = require("tailwindcss/colors");

module.exports = {
	content: ["./appview/pages/templates/**/*.html", "./appview/pages/chroma.go"],
	darkMode: "media",
	theme: {
		container: {
			padding: "2rem",
			center: true,
			screens: {
				sm: "500px",
				md: "600px",
				lg: "800px",
				xl: "1000px",
				"2xl": "1200px"
			},
		},
		extend: {
			fontFamily: {
				sans: ["iA Writer Quattro S", "Inter", "system-ui", "sans-serif", "ui-sans-serif"],
				mono: ["iA Writer Mono S", "ui-monospace", "SFMono-Regular", "Menlo", "Monaco", "Consolas", "Liberation Mono", "Courier New", "monospace"],
			},
			typography: {
				DEFAULT: {
					css: {
						maxWidth: 'none',
						pre: {
							backgroundColor: colors.gray[100],
							color: colors.black,
  						'@apply dark:bg-gray-900 dark:text-gray-300 dark:border-gray-700 dark:border': {}	
						},
					},
				},
			},
		},
	},
	plugins: [
		require('@tailwindcss/typography'),
	]
};
