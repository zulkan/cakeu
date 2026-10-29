import type { Meta, StoryObj } from '@storybook/react-vite';
import { BrowserRouter } from 'react-router-dom';
import Login from './Login';
import { useAuthStore } from '../stores/authStore';

const meta: Meta<typeof Login> = {
  title: 'Pages/Login',
  component: Login,
  decorators: [
    (Story) => (
      <BrowserRouter>
        <Story />
      </BrowserRouter>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof Login>;

export const Default: Story = {};

export const WithError: Story = {
  decorators: [
    (Story) => {
      useAuthStore.setState({ error: 'Invalid email or password' });
      return <Story />;
    },
  ],
};
