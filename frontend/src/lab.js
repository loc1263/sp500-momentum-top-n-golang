import { mount } from 'svelte';
import './app.css';
import Lab from './Lab.svelte';

export default mount(Lab, { target: document.getElementById('app') });
