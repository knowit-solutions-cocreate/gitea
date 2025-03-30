import {createApp} from 'vue';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import CatalogBranchTagSelector from '../components/CatalogBranchTagSelector.vue';

export function initCatalogBranchTagSelector() {
  // Register for elements with data-init="initCatalogBranchTagSelector"
  registerGlobalInitFunc('initCatalogBranchTagSelector', async (elRoot: HTMLElement) => {
    const app = createApp(CatalogBranchTagSelector, {elRoot});
    app.mount(elRoot);
  });

  // Initialize modal handlers
  for (const el of document.querySelectorAll('.show-catalog-ref-modal')) {
    el.addEventListener('click', () => {
      const modalTarget = el.getAttribute('data-modal');
      const currentRef = el.getAttribute('data-current-ref');
      
      const modal = fomanticQuery(modalTarget);
      if (currentRef) {
        modal.find('input[name=ref]').val(currentRef);
      }
      modal.modal('show');
    });
  }
} 