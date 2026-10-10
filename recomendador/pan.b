	switch (t->back) {
	default: Uerror("bad return move");
	case  0: goto R999; /* nothing to undo */

		 /* PROC Worker */

	case 3: // STATE 2
		;
		((P0 *)_this)->item_id = trpt->bup.oval;
		;
		goto R999;

	case 4: // STATE 3
		;
		((P0 *)_this)->item_id = trpt->bup.oval;
		;
		goto R999;

	case 5: // STATE 4
		;
		((P0 *)_this)->item_id = trpt->bup.oval;
		;
		goto R999;

	case 6: // STATE 8
		;
		now.locks[ Index(((P0 *)_this)->item_id, 3) ] = trpt->bup.oval;
		;
		goto R999;

	case 7: // STATE 10
		;
		now.in_critical_section[ Index(((P0 *)_this)->item_id, 3) ] = trpt->bup.oval;
		;
		goto R999;
;
		;
		
	case 9: // STATE 12
		;
		now.in_critical_section[ Index(((P0 *)_this)->item_id, 3) ] = trpt->bup.oval;
		;
		goto R999;

	case 10: // STATE 13
		;
		now.locks[ Index(((P0 *)_this)->item_id, 3) ] = trpt->bup.oval;
		;
		goto R999;

	case 11: // STATE 17
		;
		p_restor(II);
		;
		;
		goto R999;
	}

