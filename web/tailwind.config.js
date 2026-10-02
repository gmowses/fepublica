/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  darkMode: "class",
  theme: {
    extend: {
      colors: {
        bg: {
          DEFAULT: "rgb(var(--color-bg) / <alpha-value>)",
          soft: "rgb(var(--color-bg-soft) / <alpha-value>)",
          card: "rgb(var(--color-bg-card) / <alpha-value>)",
        },
        ink: {
          DEFAULT: "rgb(var(--color-ink) / <alpha-value>)",
          dim: "rgb(var(--color-ink-dim) / <alpha-value>)",
        },
        accent: "rgb(var(--color-accent) / <alpha-value>)",
        ok: "rgb(var(--color-ok) / <alpha-value>)",
        danger: "rgb(var(--color-danger) / <alpha-value>)",
        warn: "rgb(var(--color-warn) / <alpha-value>)",
      },
      fontFamily: {
        sans: ["IBM Plex Sans", "ui-sans-serif", "system-ui", "sans-serif"],
        display: [
          "Archivo",
          "IBM Plex Sans",
          "ui-sans-serif",
          "system-ui",
          "sans-serif",
        ],
        mono: [
          "ui-monospace",
          "SFMono-Regular",
          "SF Mono",
          "Menlo",
          "Consolas",
          "Liberation Mono",
          "monospace",
        ],
      },
    },
  },
  plugins: [],
};
