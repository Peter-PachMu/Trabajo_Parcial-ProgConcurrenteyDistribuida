#define rand	pan_rand
#define pthread_equal(a,b)	((a)==(b))
#if defined(HAS_CODE) && defined(VERBOSE)
	#ifdef BFS_PAR
		bfs_printf("Pr: %d Tr: %d\n", II, t->forw);
	#else
		cpu_printf("Pr: %d Tr: %d\n", II, t->forw);
	#endif
#endif
	switch (t->forw) {
	default: Uerror("bad forward move");
	case 0:	/* if without executable clauses */
		continue;
	case 1: /* generic 'goto' or 'skip' */
		IfNotBlocked
		_m = 3; goto P999;
	case 2: /* generic 'else' */
		IfNotBlocked
		if (trpt->o_pm&1) continue;
		_m = 3; goto P999;

		 /* PROC Worker */
	case 3: // STATE 2 - modelo.pml:17 - [item_id = 0] (0:0:1 - 1)
		IfNotBlocked
		reached[0][2] = 1;
		(trpt+1)->bup.oval = ((int)((P0 *)_this)->item_id);
		((P0 *)_this)->item_id = 0;
#ifdef VAR_RANGES
		logval("Worker:item_id", ((int)((P0 *)_this)->item_id));
#endif
		;
		_m = 3; goto P999; /* 0 */
	case 4: // STATE 3 - modelo.pml:18 - [item_id = 1] (0:0:1 - 1)
		IfNotBlocked
		reached[0][3] = 1;
		(trpt+1)->bup.oval = ((int)((P0 *)_this)->item_id);
		((P0 *)_this)->item_id = 1;
#ifdef VAR_RANGES
		logval("Worker:item_id", ((int)((P0 *)_this)->item_id));
#endif
		;
		_m = 3; goto P999; /* 0 */
	case 5: // STATE 4 - modelo.pml:19 - [item_id = 2] (0:0:1 - 1)
		IfNotBlocked
		reached[0][4] = 1;
		(trpt+1)->bup.oval = ((int)((P0 *)_this)->item_id);
		((P0 *)_this)->item_id = 2;
#ifdef VAR_RANGES
		logval("Worker:item_id", ((int)((P0 *)_this)->item_id));
#endif
		;
		_m = 3; goto P999; /* 0 */
	case 6: // STATE 7 - modelo.pml:24 - [((locks[item_id]==0))] (10:0:1 - 1)
		IfNotBlocked
		reached[0][7] = 1;
		if (!((((int)now.locks[ Index(((int)((P0 *)_this)->item_id), 3) ])==0)))
			continue;
		/* merge: locks[item_id] = 1(0, 8, 10) */
		reached[0][8] = 1;
		(trpt+1)->bup.oval = ((int)now.locks[ Index(((int)((P0 *)_this)->item_id), 3) ]);
		now.locks[ Index(((P0 *)_this)->item_id, 3) ] = 1;
#ifdef VAR_RANGES
		logval("locks[Worker:item_id]", ((int)now.locks[ Index(((int)((P0 *)_this)->item_id), 3) ]));
#endif
		;
		_m = 3; goto P999; /* 1 */
	case 7: // STATE 10 - modelo.pml:29 - [in_critical_section[item_id] = (in_critical_section[item_id]+1)] (0:0:1 - 1)
		IfNotBlocked
		reached[0][10] = 1;
		(trpt+1)->bup.oval = ((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ]);
		now.in_critical_section[ Index(((P0 *)_this)->item_id, 3) ] = (((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ])+1);
#ifdef VAR_RANGES
		logval("in_critical_section[Worker:item_id]", ((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ]));
#endif
		;
		_m = 3; goto P999; /* 0 */
	case 8: // STATE 11 - modelo.pml:32 - [assert((in_critical_section[item_id]==1))] (0:0:0 - 1)
		IfNotBlocked
		reached[0][11] = 1;
		spin_assert((((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ])==1), "(in_critical_section[item_id]==1)", II, tt, t);
		_m = 3; goto P999; /* 0 */
	case 9: // STATE 12 - modelo.pml:34 - [in_critical_section[item_id] = (in_critical_section[item_id]-1)] (0:0:1 - 1)
		IfNotBlocked
		reached[0][12] = 1;
		(trpt+1)->bup.oval = ((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ]);
		now.in_critical_section[ Index(((P0 *)_this)->item_id, 3) ] = (((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ])-1);
#ifdef VAR_RANGES
		logval("in_critical_section[Worker:item_id]", ((int)now.in_critical_section[ Index(((int)((P0 *)_this)->item_id), 3) ]));
#endif
		;
		_m = 3; goto P999; /* 0 */
	case 10: // STATE 13 - modelo.pml:38 - [locks[item_id] = 0] (0:0:1 - 1)
		IfNotBlocked
		reached[0][13] = 1;
		(trpt+1)->bup.oval = ((int)now.locks[ Index(((int)((P0 *)_this)->item_id), 3) ]);
		now.locks[ Index(((P0 *)_this)->item_id, 3) ] = 0;
#ifdef VAR_RANGES
		logval("locks[Worker:item_id]", ((int)now.locks[ Index(((int)((P0 *)_this)->item_id), 3) ]));
#endif
		;
		_m = 3; goto P999; /* 0 */
	case 11: // STATE 17 - modelo.pml:40 - [-end-] (0:0:0 - 1)
		IfNotBlocked
		reached[0][17] = 1;
		if (!delproc(1, II)) continue;
		_m = 3; goto P999; /* 0 */
	case  _T5:	/* np_ */
		if (!((!(trpt->o_pm&4) && !(trpt->tau&128))))
			continue;
		/* else fall through */
	case  _T2:	/* true */
		_m = 3; goto P999;
#undef rand
	}

