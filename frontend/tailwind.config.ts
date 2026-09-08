import type { Config } from 'tailwindcss';

const config: Config = {
  content: [
    './app/**/*.{ts,tsx}',
    './components/**/*.{ts,tsx}',
    './lib/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#eef4ff',
          100: '#d9e5ff',
          200: '#b8ccff',
          300: '#8aa9ff',
          400: '#5a7dfa',
          500: '#3a57e8',
          600: '#2c40c4',
          700: '#25349e',
          800: '#232f7d',
          900: '#212c63',
        },
      },
      fontFamily: {
        sans: [
          'system-ui',
          '-apple-system',
          '"Hiragino Kaku Gothic ProN"',
          '"Hiragino Sans"',
          'Meiryo',
          'sans-serif',
        ],
      },
    },
  },
  plugins: [],
};

export default config;
