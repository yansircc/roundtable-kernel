import { mount } from 'svelte';
import App from './App.svelte';
import './app.scss';

mount(App, {
  target: document.getElementById('app'),
});
