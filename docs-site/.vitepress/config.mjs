import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'ActionPhase Documentation',
  description: 'Complete guide for players, GMs, and developers',
  base: '/docs/',
  ignoreDeadLinks: true,  // Allow dead links during development

  themeConfig: {
    nav: [
      { text: 'Guide', link: '/guide/' },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Oriented',
          items: [
            { text: 'Overview', link: '/guide/' },
            { text: 'Getting Started', link: '/guide/getting-started' },
            { text: 'Notifications', link: '/guide/notifications' },
            { text: 'User Settings', link: '/guide/user-settings' },
            { text: 'User Profiles', link: '/guide/user-profiles' },
            { text: 'Communities', link: '/guide/communities' },
          ]
        },
        {
          text: 'Games',
          items: [
            { text: 'Game States', link: '/guide/game-states' },
            { text: 'Game Settings', link: '/guide/game-settings' },
            { text: 'Player Applications', link: '/guide/player-applications' },
            { text: 'Deadlines', link: '/guide/deadlines' },
            { text: 'Public Game Archive', link: '/guide/public-archive' },
          ]
        },
        {
          text: 'Playing',
          items: [
            { text: 'Characters', link: '/guide/characters' },
            { text: 'Character Profiles', link: '/guide/character-profile' },
            { text: 'Common Room', link: '/guide/common-room' },
            { text: 'Handouts', link: '/guide/handouts' },
            { text: 'Private Messages', link: '/guide/private-messages' },
            { text: 'Action Phases', link: '/guide/action-phases' },
            { text: 'Audience', link: '/guide/audience' },
            { text: 'History', link: '/guide/history' },
          ]
        },
        {
          text: 'Running a Game',
          items: [
            { text: 'Phases', link: '/guide/phases' },
          ]
        }
      ],
    },

    search: {
      provider: 'local'
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/RallinaTricolor/actionphase' }
    ],

    footer: {
      message: 'Released under the ISC License.',
      copyright: 'Copyright © 2025 ActionPhase'
    }
  }
})
