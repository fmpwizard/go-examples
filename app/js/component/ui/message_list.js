define(function (require) {

  'use strict';

  /**
   * Module dependencies
   */

  var defineComponent = require('flight/lib/component');

  /**
   * Module exports
   */

  return defineComponent(messageList);

  /**
   * Module function
   */

  function messageList() {
    this.defaultAttrs({
      listSelector: '.f-message-row'
    });

    this.handleDataMessageSaved = function(event, payload){
      this.addMessageLine(payload.message, {append: true});
    };

    this.addMessageLine = function (payload, append) {
      var $messageRow = this.select('listSelector').first();
      var $clonedMessageRow = $messageRow.clone().removeAttr('id').removeClass('hidden');
      $clonedMessageRow.children('.f-message').first().text(payload.body);
      $clonedMessageRow.children('.f-time').first().text(new Date(payload.createdOn));
      if (append.append === true){
        this.$node.append($clonedMessageRow);
      } else {
        this.$node.prepend($clonedMessageRow);
      }
      
    };

    this.handleDataMessages = function (event, payload) {
      if(payload.prepend){
        payload.messages.reverse().forEach(this.addMessageLine, this);
      } else {
        payload.messages.forEach(this.addMessageLine, this);
      }
      
    };

    this.after('initialize', function () {
      if ( this.$node.data('fetch-items') === true ) {
        this.trigger(document, 'uiNeedsMessages');
      }
      this.on(document, 'dataMessageSaved', this.handleDataMessageSaved);
      this.on(document, 'dataMessages', this.handleDataMessages);
    });
  }

});
